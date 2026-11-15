package config

import "gopkg.in/yaml.v3"

// QuickInitSeed writes a minimal working runners.yml at path for one runner
// on one host — the `gh sr init --quick` output. The seed composes the same
// node builders AddHost / AddRunnerFull append (newHostEntry / newRunnerEntry),
// so the seed and the mutation interface cannot drift: the seed previously
// hand-rolled its own printf'd YAML dialect, and a shape change only surfaced
// interactively at the post-write Load check.
//
// It builds the whole document in memory and writes once — the mutate API
// validates after every incremental write, and a fresh config has no runner
// until the runner entry lands, so host-first or runner-first both fail the
// intermediate check. The final writeYAMLBack validates the complete file
// with Load: a returned error means the file does not parse, which given the
// shared builders indicates a config-package bug rather than user input.
// The interactive prompt loop and the host-name/runner-name derivation stay
// with the caller.
func QuickInitSeed(path, hostName, addr string, runner AddRunnerOpts) error {
	top := &yaml.Node{Kind: yaml.MappingNode}
	hostsNode := &yaml.Node{Kind: yaml.MappingNode}
	hostsNode.Content = append(hostsNode.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: hostName},
		newHostEntry(addr, "", ""),
	)
	runnersNode := &yaml.Node{Kind: yaml.SequenceNode}
	runnersNode.Content = append(runnersNode.Content, newRunnerEntry(runner))
	top.Content = append(top.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "hosts"},
		hostsNode,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "runners"},
		runnersNode,
	)
	return writeYAMLBack(path, top)
}
