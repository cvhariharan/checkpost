package core

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"golang.org/x/sync/errgroup"
)

const (
	dashboardTopDefault = 5
	dashboardTopMax     = 20
)

// DashboardOverview assembles the machine overview dashboard payload
func (c *Core) DashboardOverview(ctx context.Context, top int) (models.DashboardOverview, error) {
	if top <= 0 {
		top = dashboardTopDefault
	}
	if top > dashboardTopMax {
		top = dashboardTopMax
	}

	now := time.Now().UTC()
	onlineCutoff := now.Add(-c.heartbeatThreshold)
	staleCutoff := now.Add(-c.policyStaleAfter)
	topN := int32(top)

	var (
		nodeCounts     repo.DashboardNodeCountsRow
		platformCounts []repo.DashboardNodeCountsByPlatformRow
		policyRows     repo.DashboardPolicyRowCountsRow
		topFailing     []repo.DashboardTopFailingPoliciesRow
		leastCompliant []repo.DashboardLeastCompliantNodesRow
		mostCompliant  []repo.DashboardMostCompliantNodesRow
		firingBySev    []repo.DashboardFiringAlertsBySeverityRow
		firingList     []repo.DashboardFiringAlertsRow
		recentMatches  []repo.DashboardRecentYaraMatchesRow
		recentEnrolled []repo.DashboardRecentEnrollmentsRow
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		nodeCounts, err = c.store.DashboardNodeCounts(gctx, onlineCutoff)
		return err
	})
	g.Go(func() (err error) {
		platformCounts, err = c.store.DashboardNodeCountsByPlatform(gctx, onlineCutoff)
		return err
	})
	g.Go(func() (err error) {
		policyRows, err = c.store.DashboardPolicyRowCounts(gctx, staleCutoff)
		return err
	})
	g.Go(func() (err error) {
		topFailing, err = c.store.DashboardTopFailingPolicies(gctx, repo.DashboardTopFailingPoliciesParams{StaleCutoff: staleCutoff, TopN: topN})
		return err
	})
	g.Go(func() (err error) {
		leastCompliant, err = c.store.DashboardLeastCompliantNodes(gctx, repo.DashboardLeastCompliantNodesParams{StaleCutoff: staleCutoff, TopN: topN})
		return err
	})
	g.Go(func() (err error) {
		mostCompliant, err = c.store.DashboardMostCompliantNodes(gctx, repo.DashboardMostCompliantNodesParams{StaleCutoff: staleCutoff, TopN: topN})
		return err
	})
	g.Go(func() (err error) {
		firingBySev, err = c.store.DashboardFiringAlertsBySeverity(gctx)
		return err
	})
	g.Go(func() (err error) {
		firingList, err = c.store.DashboardFiringAlerts(gctx, topN)
		return err
	})
	g.Go(func() (err error) {
		recentMatches, err = c.store.DashboardRecentYaraMatches(gctx, topN)
		return err
	})
	g.Go(func() (err error) {
		recentEnrolled, err = c.store.DashboardRecentEnrollments(gctx, topN)
		return err
	})
	if err := g.Wait(); err != nil {
		return models.DashboardOverview{}, fmt.Errorf("dashboard aggregates: %w", err)
	}

	out := models.DashboardOverview{
		GeneratedAt:               now,
		HeartbeatThresholdSeconds: int(c.heartbeatThreshold / time.Second),
		Machines: models.DashboardMachines{
			Total:         int(nodeCounts.Total),
			Online:        int(nodeCounts.Online),
			Offline:       int(nodeCounts.Total - nodeCounts.Online),
			NeverReported: int(nodeCounts.NeverReported),
			ByPlatform:    make([]models.DashboardPlatformCount, 0, len(platformCounts)),
		},
		Compliance: models.DashboardCompliance{
			Score: complianceScore(policyRows.WeightedPassing, policyRows.WeightedTotal),
			PolicyRows: models.DashboardPolicyRows{
				Passing: int(policyRows.Passing),
				Failing: int(policyRows.Failing),
				Unknown: int(policyRows.Unknown),
			},
			TopFailingPolicies: make([]models.DashboardFailingPolicy, 0, len(topFailing)),
			LeastCompliant:     make([]models.DashboardComplianceNode, 0, len(leastCompliant)),
			MostCompliant:      make([]models.DashboardComplianceNode, 0, len(mostCompliant)),
		},
		Security: models.DashboardSecurity{
			FiringAlerts:      firingAlerts(firingBySev),
			FiringAlertList:   make([]models.DashboardFiringAlert, 0, len(firingList)),
			RecentYaraMatches: make([]models.DashboardYaraMatch, 0, len(recentMatches)),
		},
		RecentlyEnrolled: make([]models.DashboardEnrolledMachine, 0, len(recentEnrolled)),
	}

	for _, p := range platformCounts {
		out.Machines.ByPlatform = append(out.Machines.ByPlatform, models.DashboardPlatformCount{
			Platform: p.Platform,
			Total:    int(p.Total),
			Online:   int(p.Online),
		})
	}
	for _, p := range topFailing {
		out.Compliance.TopFailingPolicies = append(out.Compliance.TopFailingPolicies, models.DashboardFailingPolicy{
			UUID:         p.Uuid.String(),
			Name:         p.Name,
			FailingCount: int(p.FailingCount),
			Platform:     p.Platform,
		})
	}
	for _, n := range leastCompliant {
		out.Compliance.LeastCompliant = append(out.Compliance.LeastCompliant, complianceNode(n.Uuid.String(), n.Hostname, n.DisplayName, n.Passing, n.Failing, n.Total, n.WeightedPassing, n.WeightedTotal))
	}
	for _, n := range mostCompliant {
		out.Compliance.MostCompliant = append(out.Compliance.MostCompliant, complianceNode(n.Uuid.String(), n.Hostname, n.DisplayName, n.Passing, n.Failing, n.Total, n.WeightedPassing, n.WeightedTotal))
	}
	for _, a := range firingList {
		out.Security.FiringAlertList = append(out.Security.FiringAlertList, models.DashboardFiringAlert{
			UUID:       a.Uuid.String(),
			Name:       a.Name,
			Severity:   a.Severity,
			Count:      int(a.Count),
			LastSeenAt: a.LastSeenAt,
		})
	}
	for _, m := range recentMatches {
		out.Security.RecentYaraMatches = append(out.Security.RecentYaraMatches, models.DashboardYaraMatch{
			ScanUUID:    m.ScanUuid.String(),
			MachineUUID: m.NodeUuid.String(),
			Hostname:    dashboardHostname(m.Hostname, m.DisplayName),
			Path:        m.Path,
			Rules:       m.Matches,
			MatchedAt:   m.CreatedAt,
		})
	}
	for _, e := range recentEnrolled {
		out.RecentlyEnrolled = append(out.RecentlyEnrolled, models.DashboardEnrolledMachine{
			UUID:        e.Uuid.String(),
			Hostname:    e.Hostname,
			DisplayName: dashboardHostname(e.Hostname, e.DisplayName),
			EnrolledAt:  e.EnrolledAt,
		})
	}

	return out, nil
}

// complianceScore returns round(100*passing/total), or nil when no policies are
// assigned (total == 0) so the UI can render an em-dash rather than a misleading 0.
func complianceScore(passing, total int64) *int {
	if total <= 0 {
		return nil
	}
	score := int(math.Round(100 * float64(passing) / float64(total)))
	return &score
}

// weightedComplianceScore returns round(100*weightedPassing/weightedTotal), or
// nil when no policies are assigned (weightedTotal == 0) so callers can render an
// em-dash rather than a misleading 0.
func weightedComplianceScore(weightedPassing, weightedTotal int64) *int {
	if weightedTotal <= 0 {
		return nil
	}
	score := int(math.Round(100 * float64(weightedPassing) / float64(weightedTotal)))
	return &score
}

func complianceNode(uuid, hostname, displayName string, passing, failing, total, weightedPassing, weightedTotal int64) models.DashboardComplianceNode {
	score := 0
	if s := weightedComplianceScore(weightedPassing, weightedTotal); s != nil {
		score = *s
	}
	return models.DashboardComplianceNode{
		UUID:        uuid,
		Hostname:    hostname,
		DisplayName: dashboardHostname(hostname, displayName),
		Score:       score,
		Passing:     int(passing),
		Failing:     int(failing),
		Total:       int(total),
	}
}

func firingAlerts(rows []repo.DashboardFiringAlertsBySeverityRow) models.DashboardFiringAlerts {
	out := models.DashboardFiringAlerts{}
	for _, r := range rows {
		n := int(r.Count)
		out.Total += n
		switch r.Severity {
		case "critical":
			out.Critical = n
		case "high":
			out.High = n
		case "medium":
			out.Medium = n
		case "low":
			out.Low = n
		case "info":
			out.Info = n
		}
	}
	return out
}

func dashboardHostname(hostname, displayName string) string {
	if displayName != "" {
		return displayName
	}
	return hostname
}
