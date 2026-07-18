package monitoring

import (
	"context"
	"testing"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/usage"
)

func TestAnalyticsPreservesRequestedHistoricalWindow(t *testing.T) {
	db := newMonitoringTestStore(t)
	ctx := context.Background()
	nowMS := int64(1_778_400_000_000)
	fourteenDaysMS := int64(14 * 24 * 60 * 60 * 1000)
	thirtyDaysMS := int64(30 * 24 * 60 * 60 * 1000)

	inside14d := monitoringEvent("inside-14d", nowMS-fourteenDaysMS+1, "glm-5.1", "auth-zhipu-a", "source-zhipu-a", false, 10, 5, 0, 0, 15, nil)
	inside14d.Provider = "zhipu"
	inside14d.AuthProviderSnapshot = "zhipu"
	inside14d.AccountSnapshot = "zhu***@gmail.com"
	inside30d := monitoringEvent("inside-30d", nowMS-thirtyDaysMS+1, "glm-5v-turbo", "auth-zhipu-b", "source-zhipu-b", false, 20, 6, 0, 0, 26, nil)
	inside30d.Provider = "zhipu"
	inside30d.AuthProviderSnapshot = "zhipu"
	inside30d.AccountSnapshot = "zhu***@gmail.com"
	outside30d := monitoringEvent("outside-30d", nowMS-thirtyDaysMS-1, "glm-4", "auth-zhipu-c", "source-zhipu-c", false, 100, 100, 0, 0, 200, nil)
	outside30d.Provider = "zhipu"
	outside30d.AuthProviderSnapshot = "zhipu"
	outside30d.AccountSnapshot = "zhu***@gmail.com"
	if _, err := db.InsertEvents(ctx, []usage.Event{inside14d, inside30d, outside30d}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	resp, err := New(db).Analytics(ctx, Request{
		FromMS: nowMS - fourteenDaysMS,
		ToMS:   nowMS,
		NowMS:  nowMS,
		Include: Include{
			Summary:      true,
			AccountStats: true,
			EventsPage:   &EventsPage{Limit: 10},
		},
	})
	if err != nil {
		t.Fatalf("14d analytics: %v", err)
	}
	if resp.Summary == nil || resp.Summary.TotalCalls != 1 || resp.Summary.TotalTokens != 15 {
		t.Fatalf("14d summary = %#v", resp.Summary)
	}
	if resp.Events == nil || len(resp.Events.Items) != 1 || resp.Events.Items[0].EventHash != "inside-14d" {
		t.Fatalf("14d events = %#v", resp.Events)
	}
	if len(resp.AccountStats) != 1 || resp.AccountStats[0].AuthProviderSnapshot != "zhipu" {
		t.Fatalf("14d account stats = %#v", resp.AccountStats)
	}

	resp, err = New(db).Analytics(ctx, Request{
		FromMS: nowMS - thirtyDaysMS,
		ToMS:   nowMS,
		NowMS:  nowMS,
		Include: Include{
			Summary:      true,
			AccountStats: true,
			EventsPage:   &EventsPage{Limit: 10},
		},
	})
	if err != nil {
		t.Fatalf("30d analytics: %v", err)
	}
	if resp.Summary == nil || resp.Summary.TotalCalls != 2 || resp.Summary.TotalTokens != 41 {
		t.Fatalf("30d summary = %#v", resp.Summary)
	}
	if resp.Events == nil || len(resp.Events.Items) != 2 {
		t.Fatalf("30d events = %#v", resp.Events)
	}
	for _, item := range resp.Events.Items {
		if item.EventHash == "outside-30d" {
			t.Fatalf("30d window included older event: %#v", resp.Events)
		}
	}
}

func TestAnalyticsNormalizesGenericAPIKeyProviderFromModel(t *testing.T) {
	db := newMonitoringTestStore(t)
	ctx := context.Background()
	fromMS := int64(1_778_250_000_000)
	toMS := fromMS + 60*60*1000

	event := monitoringEvent("legacy-mimo-provider", fromMS+1_000, "mimo-v2.5-pro", "mimo-auth", "source-mimo", false, 10, 5, 0, 0, 15, nil)
	event.Provider = "apikey"
	event.ExecutorType = "apikey"
	event.AuthType = "apikey"
	event.AuthProviderSnapshot = "apikey"
	event.AccountSnapshot = "m:tp-c...6fyc"
	event.AuthLabelSnapshot = "m:tp-c...6fyc"
	event.Source = "m:tp-c...6fyc"
	event.APIKeyHash = "client-key-mimo"
	if _, err := db.InsertEvents(ctx, []usage.Event{event}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	resp, err := New(db).Analytics(ctx, Request{
		FromMS: fromMS,
		ToMS:   toMS,
		Include: Include{
			AccountStats:  true,
			APIKeyStats:   true,
			ChannelShare:  true,
			FilterOptions: true,
			EventsPage:    &EventsPage{Limit: 10},
		},
	})
	if err != nil {
		t.Fatalf("analytics: %v", err)
	}
	if len(resp.AccountStats) != 1 || resp.AccountStats[0].AuthProviderSnapshot != "mimo" {
		t.Fatalf("account stats = %#v", resp.AccountStats)
	}
	if len(resp.APIKeyStats) != 1 || resp.APIKeyStats[0].AuthProviderSnapshot != "mimo" {
		t.Fatalf("api key stats = %#v", resp.APIKeyStats)
	}
	if len(resp.ChannelShare) != 1 || resp.ChannelShare[0].AuthProviderSnapshot != "mimo" {
		t.Fatalf("channel share = %#v", resp.ChannelShare)
	}
	if resp.FilterOptions == nil || len(resp.FilterOptions.AccountStats) != 1 || resp.FilterOptions.AccountStats[0].AuthProviderSnapshot != "mimo" {
		t.Fatalf("filter options = %#v", resp.FilterOptions)
	}
	if resp.Events == nil || len(resp.Events.Items) != 1 || resp.Events.Items[0].AuthProviderSnapshot != "mimo" {
		t.Fatalf("events = %#v", resp.Events)
	}

	resp, err = New(db).Analytics(ctx, Request{
		FromMS:  fromMS,
		ToMS:    toMS,
		Filters: Filters{Providers: []string{"mimo"}},
		Include: Include{Summary: true, EventsPage: &EventsPage{Limit: 10}},
	})
	if err != nil {
		t.Fatalf("analytics provider filter: %v", err)
	}
	if resp.Summary == nil || resp.Summary.TotalCalls != 1 || resp.Events == nil || len(resp.Events.Items) != 1 {
		t.Fatalf("provider-filtered response = %#v", resp)
	}
}

func TestAnalyticsUsesModelProviderAndSourceScopedAccountRows(t *testing.T) {
	db := newMonitoringTestStore(t)
	ctx := context.Background()
	fromMS := int64(1_778_260_000_000)
	toMS := fromMS + 60*60*1000

	first := monitoringEvent("legacy-zhipu-a", fromMS+1_000, "glm-5.1", "auth-a", "source-a", false, 10, 5, 0, 0, 15, nil)
	first.Provider = "zhipu"
	first.AuthProviderSnapshot = "claude"
	first.AccountSnapshot = "zhu***@gmail.com"
	first.AuthLabelSnapshot = "zhu***@gmail.com"
	first.Source = "zhu***@gmail.com"
	second := monitoringEvent("legacy-zhipu-b", fromMS+2_000, "glm-5v-turbo", "auth-b", "source-b", false, 20, 6, 0, 0, 26, nil)
	second.Provider = "zhipu"
	second.AuthProviderSnapshot = "claude"
	second.AccountSnapshot = "zhu***@gmail.com"
	second.AuthLabelSnapshot = "zhu***@gmail.com"
	second.Source = "zhu***@gmail.com"
	if _, err := db.InsertEvents(ctx, []usage.Event{first, second}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	resp, err := New(db).Analytics(ctx, Request{
		FromMS: fromMS,
		ToMS:   toMS,
		Include: Include{
			AccountStats: true,
			EventsPage:   &EventsPage{Limit: 10},
		},
	})
	if err != nil {
		t.Fatalf("analytics: %v", err)
	}
	if len(resp.AccountStats) != 2 {
		t.Fatalf("account stats should stay source-scoped, got %#v", resp.AccountStats)
	}
	seenIDs := map[string]struct{}{}
	for _, row := range resp.AccountStats {
		if row.AuthProviderSnapshot != "zhipu" {
			t.Fatalf("account provider = %q, want zhipu in %#v", row.AuthProviderSnapshot, row)
		}
		if row.AccountSnapshot != "zhu***@gmail.com" {
			t.Fatalf("account snapshot = %q", row.AccountSnapshot)
		}
		if len(row.SourceHashes) != 1 {
			t.Fatalf("source hashes = %#v", row.SourceHashes)
		}
		seenIDs[row.ID] = struct{}{}
	}
	if len(seenIDs) != 2 {
		t.Fatalf("account ids should differ by source hash, got %#v", resp.AccountStats)
	}
	if resp.Events == nil || len(resp.Events.Items) != 2 {
		t.Fatalf("events = %#v", resp.Events)
	}
	for _, item := range resp.Events.Items {
		if item.AuthProviderSnapshot != "zhipu" {
			t.Fatalf("event provider = %q, want zhipu in %#v", item.AuthProviderSnapshot, item)
		}
	}
}
