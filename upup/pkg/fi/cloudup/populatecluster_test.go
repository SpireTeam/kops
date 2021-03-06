package cloudup

import (
	"fmt"
	"k8s.io/kops/upup/pkg/api"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kubernetes/pkg/util/sets"
	"strings"
	"testing"
)

func buildMinimalCluster() *api.Cluster {
	c := &api.Cluster{}
	c.Name = "testcluster.test.com"
	c.Spec.Zones = []*api.ClusterZoneSpec{
		{Name: "us-east-1a", CIDR: "172.20.1.0/24"},
		{Name: "us-east-1b", CIDR: "172.20.2.0/24"},
		{Name: "us-east-1c", CIDR: "172.20.3.0/24"},
	}
	c.Spec.NetworkCIDR = "172.20.0.0/16"
	c.Spec.NonMasqueradeCIDR = "100.64.0.0/10"
	c.Spec.CloudProvider = "aws"

	// Required to stop a call to cloud provider
	// TODO: Mock cloudprovider
	c.Spec.DNSZone = "test.com"

	return c
}

func addEtcdClusters(c *api.Cluster) {
	zones := sets.NewString()
	for _, z := range c.Spec.Zones {
		zones.Insert(z.Name)
	}
	etcdZones := zones.List()

	for _, etcdCluster := range EtcdClusters {
		etcd := &api.EtcdClusterSpec{}
		etcd.Name = etcdCluster
		for _, zone := range etcdZones {
			m := &api.EtcdMemberSpec{}
			m.Name = zone
			m.Zone = zone
			etcd.Members = append(etcd.Members, m)
		}
		c.Spec.EtcdClusters = append(c.Spec.EtcdClusters, etcd)
	}
}

func TestPopulateCluster_Default_NoError(t *testing.T) {
	c := buildMinimalCluster()

	err := c.PerformAssignments()
	if err != nil {
		t.Fatalf("error from PerformAssignments: %v", err)
	}

	addEtcdClusters(c)

	registry := buildInmemoryClusterRegistry()
	_, err = PopulateClusterSpec(c, registry)
	if err != nil {
		t.Fatalf("Unexpected error from PopulateCluster: %v", err)
	}
}

func TestPopulateCluster_Docker_Spec(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.Docker = &api.DockerConfig{
		MTU:              5678,
		InsecureRegistry: "myregistry.com:1234",
	}

	err := c.PerformAssignments()
	if err != nil {
		t.Fatalf("error from PerformAssignments: %v", err)
	}

	addEtcdClusters(c)

	registry := buildInmemoryClusterRegistry()
	full, err := PopulateClusterSpec(c, registry)
	if err != nil {
		t.Fatalf("Unexpected error from PopulateCluster: %v", err)
	}

	if full.Spec.Docker.MTU != 5678 {
		t.Fatalf("Unexpected Docker MTU: %v", full.Spec.Docker.MTU)
	}

	if full.Spec.Docker.InsecureRegistry != "myregistry.com:1234" {
		t.Fatalf("Unexpected Docker InsecureRegistry: %v", full.Spec.Docker.InsecureRegistry)
	}

	// Check default values not changed
	if full.Spec.Docker.Bridge != "cbr0" {
		t.Fatalf("Unexpected Docker Bridge: %v", full.Spec.Docker.Bridge)
	}
}

func TestPopulateCluster_Custom_CIDR(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.NetworkCIDR = "172.20.2.0/24"
	c.Spec.Zones = []*api.ClusterZoneSpec{
		{Name: "us-east-1a", CIDR: "172.20.2.0/27"},
		{Name: "us-east-1b", CIDR: "172.20.2.32/27"},
		{Name: "us-east-1c", CIDR: "172.20.2.64/27"},
	}

	err := c.PerformAssignments()
	if err != nil {
		t.Fatalf("error from PerformAssignments: %v", err)
	}

	addEtcdClusters(c)

	registry := buildInmemoryClusterRegistry()
	full, err := PopulateClusterSpec(c, registry)
	if err != nil {
		t.Fatalf("Unexpected error from PopulateCluster: %v", err)
	}
	if full.Spec.NetworkCIDR != "172.20.2.0/24" {
		t.Fatalf("Unexpected NetworkCIDR: %v", full.Spec.NetworkCIDR)
	}
}

func TestPopulateCluster_IsolateMasters(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.IsolateMasters = fi.Bool(true)

	err := c.PerformAssignments()
	if err != nil {
		t.Fatalf("error from PerformAssignments: %v", err)
	}

	addEtcdClusters(c)

	registry := buildInmemoryClusterRegistry()
	full, err := PopulateClusterSpec(c, registry)
	if err != nil {
		t.Fatalf("Unexpected error from PopulateCluster: %v", err)
	}
	if fi.BoolValue(full.Spec.MasterKubelet.EnableDebuggingHandlers) != false {
		t.Fatalf("Unexpected EnableDebuggingHandlers: %v", fi.BoolValue(full.Spec.MasterKubelet.EnableDebuggingHandlers))
	}
	if fi.BoolValue(full.Spec.MasterKubelet.ReconcileCIDR) != false {
		t.Fatalf("Unexpected ReconcileCIDR: %v", fi.BoolValue(full.Spec.MasterKubelet.ReconcileCIDR))
	}
}

func TestPopulateCluster_IsolateMastersFalse(t *testing.T) {
	c := buildMinimalCluster()
	// default: c.Spec.IsolateMasters = fi.Bool(false)

	err := c.PerformAssignments()
	if err != nil {
		t.Fatalf("error from PerformAssignments: %v", err)
	}

	addEtcdClusters(c)

	registry := buildInmemoryClusterRegistry()
	full, err := PopulateClusterSpec(c, registry)
	if err != nil {
		t.Fatalf("Unexpected error from PopulateCluster: %v", err)
	}
	if fi.BoolValue(full.Spec.MasterKubelet.EnableDebuggingHandlers) != true {
		t.Fatalf("Unexpected EnableDebuggingHandlers: %v", fi.BoolValue(full.Spec.MasterKubelet.EnableDebuggingHandlers))
	}
	if fi.BoolValue(full.Spec.MasterKubelet.ReconcileCIDR) != true {
		t.Fatalf("Unexpected ReconcileCIDR: %v", fi.BoolValue(full.Spec.MasterKubelet.ReconcileCIDR))
	}
}

func TestPopulateCluster_Name_Required(t *testing.T) {
	c := buildMinimalCluster()
	c.Name = ""

	expectErrorFromPopulateCluster(t, c, "Name")
}

func TestPopulateCluster_Zone_Required(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.Zones = nil

	expectErrorFromPopulateCluster(t, c, "Zone")
}

func TestPopulateCluster_NetworkCIDR_Required(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.NetworkCIDR = ""

	expectErrorFromPopulateCluster(t, c, "NetworkCIDR")
}

func TestPopulateCluster_NonMasqueradeCIDR_Required(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.NonMasqueradeCIDR = ""

	expectErrorFromPopulateCluster(t, c, "NonMasqueradeCIDR")
}

func TestPopulateCluster_CloudProvider_Required(t *testing.T) {
	c := buildMinimalCluster()
	c.Spec.CloudProvider = ""

	expectErrorFromPopulateCluster(t, c, "CloudProvider")
}

func expectErrorFromPopulateCluster(t *testing.T, c *api.Cluster, message string) {
	registry := buildInmemoryClusterRegistry()
	_, err := PopulateClusterSpec(c, registry)
	if err == nil {
		t.Fatalf("Expected error from PopulateCluster")
	}
	actualMessage := fmt.Sprintf("%v", err)
	if !strings.Contains(actualMessage, message) {
		t.Fatalf("Expected error %q, got %q", message, actualMessage)
	}
}
