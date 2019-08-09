// XXX
// NOTE: requires SQL Server Browser service to be running

package collector

import (
	"flag"
	"fmt"
	"log"
	"regexp"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Factories["sql"] = NewSQLCollector

	// XXX enumerate all local sql services

	sqlInstances = getSQLInstances()
}

var (
	sqlInstances []string
	dbWhitelist  = flag.String("collector.sql.db-whitelist", ".+", "Regexp of databases to whitelist. Name must both match whitelist and not match blacklist to be included.")
	dbBlacklist  = flag.String("collector.sql.db-blacklist", "", "Regexp of databases to blacklist. Name must both match whitelist and not match blacklist to be included.")
)

func getSQLInstances() []string {

	// XXX 1. run:   Get-WmiObject -NameSpace root\Microsoft\SQLServer\ComputerManagement13 -Query "SELECT * FROM SqlService"
	var dst []SqlService
	q := wmi.CreateQuery(&dst, "")
	err := wmi.QueryNamespace(q, &dst, `root\Microsoft\SQLServer\ComputerManagement13`)
	if err != nil {
		// XXX 2. if fail, try different namespaces ComputerManagement10 and ComputerManagement
		log.Fatal(err)
	}

	res := []string{}
	// XXX extract names
	for _, inst := range dst {
		res = append(res, inst.ServiceName)
	}
	fmt.Println("RETURNING:", res)
	return res
}

// A SQLCollector is a Prometheus collector for WMI XXX
type SQLCollector struct {
	BytesReceivedTotal       *prometheus.Desc
	BytesSentTotal           *prometheus.Desc
	BytesTotal               *prometheus.Desc
	PacketsOutboundDiscarded *prometheus.Desc
	PacketsOutboundErrors    *prometheus.Desc
	PacketsTotal             *prometheus.Desc
	PacketsReceivedDiscarded *prometheus.Desc
	PacketsReceivedErrors    *prometheus.Desc
	PacketsReceivedTotal     *prometheus.Desc
	PacketsReceivedUnknown   *prometheus.Desc
	PacketsSentTotal         *prometheus.Desc

	nicWhitelistPattern *regexp.Regexp
	nicBlacklistPattern *regexp.Regexp
}

// NewSQLCollector ...
func NewSQLCollector() (Collector, error) {
	const subsystem = "net"

	return &SQLCollector{
		BytesReceivedTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "bytes_received_total"),
			"(Network.BytesReceivedPerSec)",
			[]string{"nic"},
			nil,
		),
		BytesSentTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "bytes_sent_total"),
			"(Network.BytesSentPerSec)",
			[]string{"nic"},
			nil,
		),
		BytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "bytes_total"),
			"(Network.BytesTotalPerSec)",
			[]string{"nic"},
			nil,
		),
		PacketsOutboundDiscarded: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_outbound_discarded"),
			"(Network.PacketsOutboundDiscarded)",
			[]string{"nic"},
			nil,
		),
		PacketsOutboundErrors: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_outbound_errors"),
			"(Network.PacketsOutboundErrors)",
			[]string{"nic"},
			nil,
		),
		PacketsReceivedDiscarded: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_received_discarded"),
			"(Network.PacketsReceivedDiscarded)",
			[]string{"nic"},
			nil,
		),
		PacketsReceivedErrors: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_received_errors"),
			"(Network.PacketsReceivedErrors)",
			[]string{"nic"},
			nil,
		),
		PacketsReceivedTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_received_total"),
			"(Network.PacketsReceivedPerSec)",
			[]string{"nic"},
			nil,
		),
		PacketsReceivedUnknown: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_received_unknown"),
			"(Network.PacketsReceivedUnknown)",
			[]string{"nic"},
			nil,
		),
		PacketsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_total"),
			"(Network.PacketsPerSec)",
			[]string{"nic"},
			nil,
		),
		PacketsSentTotal: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, subsystem, "packets_sent_total"),
			"(Network.PacketsSentPerSec)",
			[]string{"nic"},
			nil,
		),

		nicWhitelistPattern: regexp.MustCompile(fmt.Sprintf("^(?:%s)$", *nicWhitelist)),
		nicBlacklistPattern: regexp.MustCompile(fmt.Sprintf("^(?:%s)$", *nicBlacklist)),
	}, nil
}

// Collect sends the metric values for each metric
// to the provided prometheus Metric channel.
func (c *SQLCollector) Collect(ch chan<- prometheus.Metric) error {
	if desc, err := c.collect(ch); err != nil {
		log.Println("[ERROR] failed collecting sql metrics:", desc, err)
		return err
	}
	return nil
}

type SqlService struct {
	ServiceName string
}

func (c *SQLCollector) collect(ch chan<- prometheus.Metric) (*prometheus.Desc, error) {
	var dst []Win32_PerfRawData_Tcpip_NetworkInterface

	q := wmi.CreateQuery(&dst, "")
	if err := wmi.Query(q, &dst); err != nil {
		return nil, err
	}

	for _, nic := range dst {
		if c.nicBlacklistPattern.MatchString(nic.Name) ||
			!c.nicWhitelistPattern.MatchString(nic.Name) {
			continue
		}

		name := mangleNetworkName(nic.Name)
		if name == "" {
			continue
		}

		// Counters
		ch <- prometheus.MustNewConstMetric(
			c.BytesReceivedTotal,
			prometheus.CounterValue,
			float64(nic.BytesReceivedPerSec),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.BytesSentTotal,
			prometheus.CounterValue,
			float64(nic.BytesSentPerSec),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.BytesTotal,
			prometheus.CounterValue,
			float64(nic.BytesTotalPerSec),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsOutboundDiscarded,
			prometheus.CounterValue,
			float64(nic.PacketsOutboundDiscarded),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsOutboundErrors,
			prometheus.CounterValue,
			float64(nic.PacketsOutboundErrors),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsTotal,
			prometheus.CounterValue,
			float64(nic.PacketsPerSec),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsReceivedDiscarded,
			prometheus.CounterValue,
			float64(nic.PacketsReceivedDiscarded),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsReceivedErrors,
			prometheus.CounterValue,
			float64(nic.PacketsReceivedErrors),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsReceivedTotal,
			prometheus.CounterValue,
			float64(nic.PacketsReceivedPerSec),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsReceivedUnknown,
			prometheus.CounterValue,
			float64(nic.PacketsReceivedUnknown),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.PacketsSentTotal,
			prometheus.CounterValue,
			float64(nic.PacketsSentPerSec),
			name,
		)
	}

	return nil, nil
}
