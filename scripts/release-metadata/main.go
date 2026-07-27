// Command release-metadata creates deterministic checksums, an SPDX 2.3 SBOM,
// and an in-toto/SLSA provenance statement for DiffBeacon release binaries.
package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

type artifact struct {
	name   string
	path   string
	digest string
	info   *debug.BuildInfo
}

func main() {
	directory := "dist"
	if len(os.Args) == 2 {
		directory = os.Args[1]
	} else if len(os.Args) != 1 {
		fatalf("usage: go run ./scripts/release-metadata [artifact-directory]")
	}
	artifacts, err := inspectArtifacts(directory)
	if err != nil {
		fatalf("inspect artifacts: %v", err)
	}
	if err := writeChecksums(directory, artifacts); err != nil {
		fatalf("write checksums: %v", err)
	}
	if err := writeJSON(filepath.Join(directory, "diffbeacon.spdx.json"), makeSPDX(artifacts)); err != nil {
		fatalf("write SBOM: %v", err)
	}
	if err := writeJSON(filepath.Join(directory, "provenance.intoto.json"), makeProvenance(artifacts)); err != nil {
		fatalf("write provenance: %v", err)
	}
}

func inspectArtifacts(directory string) ([]artifact, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	var result []artifact
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "diffbeacon_") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		info, err := buildinfo.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s is not an inspectable Go binary: %w", path, err)
		}
		sum := sha256.Sum256(data)
		result = append(result, artifact{name: entry.Name(), path: path, digest: hex.EncodeToString(sum[:]), info: info})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	if len(result) == 0 {
		return nil, fmt.Errorf("no diffbeacon_* binaries found in %s", directory)
	}
	return result, nil
}

func writeChecksums(directory string, artifacts []artifact) error {
	var content strings.Builder
	for _, item := range artifacts {
		fmt.Fprintf(&content, "%s  %s\n", item.digest, item.name)
	}
	return os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(content.String()), 0o644)
}

func makeSPDX(artifacts []artifact) map[string]any {
	packages := make([]map[string]any, 0)
	relationships := make([]map[string]string, 0)
	modules := map[string]*debug.Module{}
	for index, item := range artifacts {
		id := fmt.Sprintf("SPDXRef-Artifact-%d", index+1)
		packages = append(packages, map[string]any{
			"SPDXID": id, "name": item.name, "versionInfo": moduleVersion(&item.info.Main),
			"downloadLocation": "NOASSERTION", "filesAnalyzed": false,
			"licenseConcluded": "MIT", "licenseDeclared": "MIT",
			"checksums": []map[string]string{{"algorithm": "SHA256", "checksumValue": item.digest}},
		})
		relationships = append(relationships, map[string]string{"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES", "relatedSpdxElement": id})
		for _, dependency := range item.info.Deps {
			modules[dependency.Path+"@"+moduleVersion(dependency)] = dependency
		}
	}
	keys := make([]string, 0, len(modules))
	for key := range modules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for index, key := range keys {
		module := modules[key]
		id := fmt.Sprintf("SPDXRef-GoModule-%d", index+1)
		packages = append(packages, map[string]any{
			"SPDXID": id, "name": module.Path, "versionInfo": moduleVersion(module),
			"downloadLocation": "NOASSERTION", "filesAnalyzed": false,
			"licenseConcluded": "NOASSERTION", "licenseDeclared": "NOASSERTION",
			"externalRefs": []map[string]string{{"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl", "referenceLocator": "pkg:golang/" + module.Path + "@" + moduleVersion(module)}},
		})
		for artifactIndex := range artifacts {
			relationships = append(relationships, map[string]string{"spdxElementId": fmt.Sprintf("SPDXRef-Artifact-%d", artifactIndex+1), "relationshipType": "DEPENDS_ON", "relatedSpdxElement": id})
		}
	}
	namespaceSeed := ""
	for _, item := range artifacts {
		namespaceSeed += item.digest
	}
	namespaceHash := sha256.Sum256([]byte(namespaceSeed))
	return map[string]any{
		"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT",
		"name": "DiffBeacon release artifacts", "documentNamespace": "https://diffbeacon.invalid/spdx/" + hex.EncodeToString(namespaceHash[:]),
		"creationInfo": map[string]any{"created": creationTime(), "creators": []string{"Tool: diffbeacon-release-metadata"}},
		"packages":     packages, "relationships": relationships,
	}
}

func makeProvenance(artifacts []artifact) map[string]any {
	subjects := make([]map[string]any, 0, len(artifacts))
	for _, item := range artifacts {
		subjects = append(subjects, map[string]any{"name": item.name, "digest": map[string]string{"sha256": item.digest}})
	}
	return map[string]any{
		"_type": "https://in-toto.io/Statement/v1", "subject": subjects,
		"predicateType": "https://slsa.dev/provenance/v1",
		"predicate": map[string]any{
			"buildDefinition": map[string]any{"buildType": "https://diffbeacon.invalid/build/go-cross/v1", "externalParameters": map[string]any{"command": "make build-all release-metadata"}},
			"runDetails":      map[string]any{"builder": map[string]string{"id": builderID()}, "metadata": map[string]string{"startedOn": creationTime()}},
		},
	}
}

func moduleVersion(module *debug.Module) string {
	if module == nil || module.Version == "" || module.Version == "(devel)" {
		return "0.0.0-devel"
	}
	return module.Version
}

func creationTime() string {
	if value := os.Getenv("SOURCE_DATE_EPOCH"); value != "" {
		seconds, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
		}
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func builderID() string {
	if repository, runID := os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID"); repository != "" && runID != "" {
		return "https://github.com/" + repository + "/actions/runs/" + runID
	}
	return "local:go-build"
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
