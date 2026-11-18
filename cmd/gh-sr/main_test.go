package main

import (
	"sort"
	"testing"

	"github.com/spf13/cobra"
)

// The per-host service command groups (cache server, registry mirrors) are
// built from one spec, so they must expose the same subcommands — the only
// documented difference is that the cache server has a management API to
// prune, which the registry has no counterpart for.
func TestHostServiceCmdGroups(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cmd  *cobra.Command
		want []string
	}{
		{name: "cache", cmd: cacheCmd(), want: []string{"deploy", "prune", "remove", "status"}},
		{name: "registry", cmd: registryCmd(), want: []string{"deploy", "remove", "status"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			for _, sub := range tc.cmd.Commands() {
				got = append(got, sub.Name())
			}
			sort.Strings(got)

			if len(got) != len(tc.want) {
				t.Fatalf("subcommands = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("subcommands = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// The remove subcommand carries the --purge-data flag in both groups.
func TestHostServiceRemoveCmd_hasPurgeDataFlag(t *testing.T) {
	t.Parallel()

	for _, cmd := range []*cobra.Command{cacheCmd(), registryCmd()} {
		remove, _, err := cmd.Find([]string{"remove"})
		if err != nil {
			t.Fatalf("%s remove: %v", cmd.Name(), err)
		}
		flag := remove.Flags().Lookup("purge-data")
		if flag == nil {
			t.Errorf("%s remove: missing --purge-data flag", cmd.Name())
			continue
		}
		if flag.DefValue != "false" {
			t.Errorf("%s remove: --purge-data default = %q, want false", cmd.Name(), flag.DefValue)
		}
	}
}

// The shared builder must not leak one service's wording into the other.
func TestHostServiceCmdGroups_keepOwnWording(t *testing.T) {
	t.Parallel()

	cache := cacheCmd()
	registry := registryCmd()

	if cache.Name() != "cache" {
		t.Errorf("cache group name = %q", cache.Name())
	}
	if registry.Name() != "registry" {
		t.Errorf("registry group name = %q", registry.Name())
	}
	if cache.Short == registry.Short {
		t.Error("the two groups share the same short description")
	}
	if cache.Long == registry.Long {
		t.Error("the two groups share the same long description")
	}

	cacheRemove, _, err := cache.Find([]string{"remove"})
	if err != nil {
		t.Fatalf("cache remove: %v", err)
	}
	registryRemove, _, err := registry.Find([]string{"remove"})
	if err != nil {
		t.Fatalf("registry remove: %v", err)
	}
	if cacheRemove.Long == registryRemove.Long {
		t.Error("the two remove subcommands share the same long description")
	}
}
