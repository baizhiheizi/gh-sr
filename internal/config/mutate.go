package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AddHost adds a host entry to the config file at cfgPath. It reads the existing
// YAML, appends the host, validates, and writes back.
func AddHost(cfgPath, name, addr, hostOS, arch string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return fmt.Errorf("config root is not a mapping")
	}

	hostsNode := findMapValue(top, "hosts")
	if hostsNode == nil {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "hosts"}
		hostsNode = &yaml.Node{Kind: yaml.MappingNode}
		top.Content = append(top.Content, keyNode, hostsNode)
	}
	if hostsNode.Kind != yaml.MappingNode {
		return fmt.Errorf("hosts is not a mapping")
	}

	if existing := findMapValue(hostsNode, name); existing != nil {
		return fmt.Errorf("host %q already exists in config", name)
	}

	hostEntry := &yaml.Node{Kind: yaml.MappingNode}
	hostEntry.Content = append(hostEntry.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "addr"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: addr},
	)
	if hostOS != "" {
		hostEntry.Content = append(hostEntry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "os"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: hostOS},
		)
	}
	if arch != "" {
		hostEntry.Content = append(hostEntry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "arch"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: arch},
		)
	}

	hostsNode.Content = append(hostsNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		hostEntry,
	)

	return writeYAMLBack(cfgPath, &root)
}

// AddRunner adds a runner entry to the config file at cfgPath.
func AddRunner(cfgPath, name, repo, hostName string, count int, labels []string, mode string) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return fmt.Errorf("config root is not a mapping")
	}

	runnersNode := findMapValue(top, "runners")
	if runnersNode == nil {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "runners"}
		runnersNode = &yaml.Node{Kind: yaml.SequenceNode}
		top.Content = append(top.Content, keyNode, runnersNode)
	}
	if runnersNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("runners is not a sequence")
	}

	entry := &yaml.Node{Kind: yaml.MappingNode}
	entry.Content = append(entry.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "name"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: name},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "repo"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: repo},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "host"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: hostName},
	)

	if count > 1 {
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "count"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%d", count), Tag: "!!int"},
		)
	}

	if len(labels) > 0 {
		labelsSeq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, l := range labels {
			labelsSeq.Content = append(labelsSeq.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: l},
			)
		}
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "labels"},
			labelsSeq,
		)
	}

	if mode != "" {
		entry.Content = append(entry.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "mode"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: mode},
		)
	}

	runnersNode.Content = append(runnersNode.Content, entry)

	return writeYAMLBack(cfgPath, &root)
}

func findMapValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func writeYAMLBack(cfgPath string, root *yaml.Node) error {
	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(cfgPath, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	if _, loadErr := Load(cfgPath); loadErr != nil {
		return fmt.Errorf("config invalid after modification: %w", loadErr)
	}

	return nil
}
