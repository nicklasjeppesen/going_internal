package outbox

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

// eventRow is a row in domain_events. It is saved through the ORM, so a row
// saved with a ctx from db.Transaction joins that transaction.
type eventRow struct {
	ActiveRecord[*eventRow]
	Name         string
	Payload      string
	User_id      int64
	Published_at string
	Claimed_at   string
	Attempts     int64
	Last_error   string
}

func (_row eventRow) DB(ctx context.Context) *eventRow {
	row := &_row
	row.Table = "domain_events"
	row.Columns = Columns{
		"name":         &row.Name,
		"payload":      &row.Payload,
		"user_id":      &row.User_id,
		"published_at": &row.Published_at,
		"claimed_at":   &row.Claimed_at,
		"attempts":     &row.Attempts,
		"last_error":   &row.Last_error,
	}
	row.ParentDB = CreateORM(ctx, row)
	return row
}

// deliveryRow is a row in event_deliveries: one event for one named listener.
type deliveryRow struct {
	ActiveRecord[*deliveryRow]
	Event_id     int64
	Listener     string
	Async        int64 // 1 = registered with events.Async
	Delivered_at string
	Claimed_at   string
	Attempts     int64
	Last_error   string
}

func (_row deliveryRow) DB(ctx context.Context) *deliveryRow {
	row := &_row
	row.Table = "event_deliveries"
	row.Columns = Columns{
		"event_id":     &row.Event_id,
		"listener":     &row.Listener,
		"async":        &row.Async,
		"delivered_at": &row.Delivered_at,
		"claimed_at":   &row.Claimed_at,
		"attempts":     &row.Attempts,
		"last_error":   &row.Last_error,
	}
	row.ParentDB = CreateORM(ctx, row)
	return row
}
