package events

import (
	"context"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/events"
	. "github.com/nicklasjeppesen/going_internal/super/tests/db/shared"
	"github.com/nicklasjeppesen/going_internal/super/tests/db/shared/data"
	model "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/models"
	schema "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/schema"
)

// Tests that every ActiveRecord model can record domain events, that the ORM
// never writes or publishes them, and that a model is an events.Recorder.

// userRenamed and userAged are small stand ins for real domain events.
type userRenamed struct {
	ID   int64
	Name string
}

func (userRenamed) EventName() string { return "user.renamed" }

type userAged struct{ ID int64 }

func (userAged) EventName() string { return "user.aged" }

// setupUsers makes a database with one user and one comment.
func setupUsers(t *testing.T) *TestDB {
	t.Helper()
	database, err := NewInMemoryDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB.Close() })

	for _, s := range []string{schema.UserSchema, schema.CommentSchema} {
		if err := database.LoadSchema(s); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec("INSERT INTO comments (body, commentable_id, commentable_type) VALUES ('Hej', 1, 'user')"); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestReleaseEventsReturnsTheRecordedEventsAndClearsThem(t *testing.T) {
	user := model.User{}.DB(context.Background())

	if got := user.ReleaseEvents(); len(got) != 0 {
		t.Fatalf("a new model must have no events, got %v", got)
	}

	user.Record(userRenamed{ID: 1, Name: "Nick"})
	user.Record(userAged{ID: 1})

	got := user.ReleaseEvents()
	if len(got) != 2 || got[0] != (userRenamed{ID: 1, Name: "Nick"}) || got[1] != (userAged{ID: 1}) {
		t.Fatalf("ReleaseEvents: got %v, want both events in the order they were recorded", got)
	}
	if again := user.ReleaseEvents(); len(again) != 0 {
		t.Fatalf("a second ReleaseEvents must be empty, got %v", again)
	}
}

func TestSaveAndUpdateNeitherWriteNorPublishTheEvents(t *testing.T) {
	database := setupUsers(t)
	ctx := context.Background()

	user := model.User{}.DB(ctx)
	user.Name = "Nick"
	user.Record(userRenamed{Name: "Nick"})

	saved, err := user.Save()
	if err != nil {
		t.Fatalf("Save with recorded events: %v", err)
	}
	saved.Age = 40
	if err := saved.Update(); err != nil {
		t.Fatalf("Update with recorded events: %v", err)
	}

	// The row holds only its own columns.
	var name string
	var age int64
	if err := database.DB.QueryRow("SELECT name, age FROM users WHERE id = ?", saved.Id).Scan(&name, &age); err != nil {
		t.Fatal(err)
	}
	if name != "Nick" || age != 40 {
		t.Fatalf("saved row: got (%s, %d), want (Nick, 40)", name, age)
	}

	// Save and Update published nothing: the event is still waiting.
	if got := user.ReleaseEvents(); len(got) != 1 {
		t.Fatalf("Save/Update must leave the events for the caller, got %v", got)
	}

	// And nothing of it is in the JSON.
	saved.Record(userAged{})
	for key := range saved.ToJson() {
		if key == "recordedEvents" || key == "RecordedEvents" {
			t.Fatalf("ToJson must not include the recorded events, got key %q", key)
		}
	}
}

func TestRowsLoadedWithFirstAndGetStartWithoutEvents(t *testing.T) {
	setupUsers(t)
	ctx := context.Background()

	// Comment's DB copies its receiver, so a row made from a model with
	// events could inherit them if the ORM didn't clear them.
	query := model.Comment{}.DB(ctx)
	query.Record(userAged{})

	for _, row := range query.Where("commentable_type", "user").Get() {
		if got := row.ReleaseEvents(); len(got) != 0 {
			t.Fatalf("a row from Get must start without events, got %v", got)
		}
	}

	query = model.Comment{}.DB(ctx)
	query.Record(userAged{})
	first := query.Where("id", 1).First()
	if first.IsEmpty() {
		t.Fatal("expected the comment")
	}
	if got := first.ReleaseEvents(); len(got) != 0 {
		t.Fatalf("a row from First must start without events, got %v", got)
	}
}

func TestDBCopyKeepsTheEventsWithoutSharingThem(t *testing.T) {
	ctx := context.Background()

	original := model.Comment{}.DB(ctx)
	original.Record(userRenamed{Name: "first"})

	copied := original.DB(ctx) // Comment's DB copies its receiver
	copied.Record(userAged{})
	original.Record(userRenamed{Name: "second"})

	if got := copied.ReleaseEvents(); len(got) != 2 || got[1] != (userAged{}) {
		t.Fatalf("the copy: got %v, want [first, aged]", got)
	}
	if got := original.ReleaseEvents(); len(got) != 2 || got[1] != (userRenamed{Name: "second"}) {
		t.Fatalf("the original: got %v, want [first, second]", got)
	}
}

// publishRecorded is what a command does after it saved a model.
func publishRecorded(ctx context.Context, publisher events.Publisher, recorder events.Recorder) error {
	return publisher.PublishAll(ctx, recorder.ReleaseEvents()...)
}

func TestAModelIsARecorder(t *testing.T) {
	setupUsers(t)
	ctx := context.Background()

	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var renamed []string
	events.On(dispatcher, func(_ context.Context, event userRenamed) error {
		renamed = append(renamed, event.Name)
		return nil
	})

	user := model.User{}.DB(ctx).Where("id", 1).First()
	user.Name = "Nick"
	user.Record(userRenamed{ID: user.Id, Name: user.Name})
	if err := user.Update(); err != nil {
		t.Fatal(err)
	}
	if len(renamed) != 0 {
		t.Fatal("Update must never publish by itself")
	}

	var recorder events.Recorder = user
	if err := publishRecorded(ctx, dispatcher, recorder); err != nil {
		t.Fatal(err)
	}
	if len(renamed) != 1 || renamed[0] != "Nick" {
		t.Fatalf("the sync listener: got %v, want [Nick]", renamed)
	}
	if err := publishRecorded(ctx, dispatcher, user); err != nil || len(renamed) != 1 {
		t.Fatalf("publishing again must publish nothing, got %v (err %v)", renamed, err)
	}
}
