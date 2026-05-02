package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAnsibleINIInventoryIncludesGroupInheritance(t *testing.T) {
	body := `
[all:vars]
ansible_user=ubuntu
ansible_port=22

[web]
web01 ansible_host=10.0.0.11
web02 ansible_host=10.0.0.12 ansible_ssh_private_key_file=~/.ssh/web.pem

[prod:children]
web
`

	inv, err := parseAnsibleINIInventory(body)
	if err != nil {
		t.Fatalf("parseAnsibleINIInventory() error = %v", err)
	}

	hosts := inv.flattenHosts()
	if len(hosts) != 2 {
		t.Fatalf("hosts = %d, want 2", len(hosts))
	}
	if hosts[0].Vars["ansible_user"] != "ubuntu" {
		t.Fatalf("ansible_user = %q", hosts[0].Vars["ansible_user"])
	}
	if hosts[1].Vars["ansible_ssh_private_key_file"] != "~/.ssh/web.pem" {
		t.Fatalf("private key = %q", hosts[1].Vars["ansible_ssh_private_key_file"])
	}
	if got := hosts[0].Groups; len(got) != 2 || got[0] != "prod" || got[1] != "web" {
		t.Fatalf("groups = %#v", got)
	}
}

func TestExpandAnsibleHostPattern(t *testing.T) {
	got := expandAnsibleHostPattern("app[01:03].example.com")
	want := []string{"app01.example.com", "app02.example.com", "app03.example.com"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestListAnsibleInventoryFromYAML(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "inventory.yaml")
	body := `
all:
  vars:
    ansible_user: ubuntu
  children:
    app:
      vars:
        ansible_port: 2222
      hosts:
        web01:
          ansible_host: 10.0.0.21
          env: prod
`
	if err := os.WriteFile(inventoryPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write inventory: %v", err)
	}

	servers, err := listAnsibleInventory(map[string]interface{}{
		"inventory_file":   inventoryPath,
		"inventory_format": "yaml",
	})
	if err != nil {
		t.Fatalf("listAnsibleInventory() error = %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(servers))
	}

	server := servers[0]
	if server.Name != "ansible:web01" {
		t.Fatalf("name = %q", server.Name)
	}
	if server.Config["addr"] != "10.0.0.21" {
		t.Fatalf("addr = %v", server.Config["addr"])
	}
	if server.Config["user"] != "ubuntu" {
		t.Fatalf("user = %v", server.Config["user"])
	}
	if server.Config["port"] != "2222" {
		t.Fatalf("port = %v", server.Config["port"])
	}
	if server.Meta["group.app"] != "true" {
		t.Fatalf("group.app meta = %q", server.Meta["group.app"])
	}
	if server.Meta["var.env"] != "prod" {
		t.Fatalf("var.env = %q", server.Meta["var.env"])
	}
}

func TestListAnsibleInventoryIncludeExcludeGroups(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "inventory.ini")
	body := `
[web]
web01 ansible_host=10.0.0.11

[db]
db01 ansible_host=10.0.0.31
`
	if err := os.WriteFile(inventoryPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write inventory: %v", err)
	}

	servers, err := listAnsibleInventory(map[string]interface{}{
		"inventory_file": inventoryPath,
		"include_groups": []interface{}{"web", "db"},
		"exclude_groups": []interface{}{"db"},
	})
	if err != nil {
		t.Fatalf("listAnsibleInventory() error = %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(servers))
	}
	if servers[0].Name != "ansible:web01" {
		t.Fatalf("name = %q", servers[0].Name)
	}
}
