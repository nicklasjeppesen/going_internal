package cases

import (
	"context"
	"testing"

	. "github.com/nicklasjeppesen/going_internal/super/tests/db/shared"
	"github.com/nicklasjeppesen/going_internal/super/tests/db/shared/data"
	model "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/models"
	schema "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/schema"
)

// Tests that HasMany, HasOne and HasManyMorph give a saved child the parent's
// id. They used to take it from the relation's empty child prototype (Holder),
// so Attach panicked with "interface {} is nil, not int64".

// setupUsers makes a database with users, posts and comments, plus one extra
// user first so the parent's id (2) can't be mistaken for a default of 1.
func setupUsers(t *testing.T) (*TestDB, *model.User) {
	t.Helper()
	database, err := NewInMemoryDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB.Close() })

	for _, s := range []string{schema.UserSchema, schema.PostSchema, schema.CommentSchema} {
		if err := database.LoadSchema(s); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := database.LoadJsonData(data.UserJSON, "users"); err != nil {
			t.Fatal(err)
		}
	}

	user := model.User{}.DB(context.Background()).Where("id", 2).First()
	if user.IsEmpty() || user.Id != 2 {
		t.Fatalf("setupUsers: expected user 2, got Id=%v IsEmpty=%v", user.Id, user.IsEmpty())
	}
	return database, user
}

// postOwner returns user_id of the only post with the given title.
func postOwner(t *testing.T, database *TestDB, title string) int64 {
	t.Helper()
	var userID int64
	if err := database.DB.QueryRow("SELECT user_id FROM posts WHERE title = ?", title).Scan(&userID); err != nil {
		t.Fatalf("postOwner(%q): %v", title, err)
	}
	return userID
}

func newPost(user *model.User, title string) *model.Post {
	post := model.Post{}.DB(user.GetCtx())
	post.Title = title
	return post
}

func TestHasManyAttachSetsParentID(t *testing.T) {
	database, user := setupUsers(t)

	if err := user.Posts().Attach(newPost(user, "Attached")); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if got := postOwner(t, database, "Attached"); got != 2 {
		t.Errorf("Attach: expected user_id=2, got %d", got)
	}
}

func TestHasManyAttachWithKeys(t *testing.T) {
	database, user := setupUsers(t)

	if err := user.Posts().ForeignKey("user_id").LocalKey("id").Attach(newPost(user, "Keys")); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if got := postOwner(t, database, "Keys"); got != 2 {
		t.Errorf("Attach with keys: expected user_id=2, got %d", got)
	}
}

func TestHasManySaveSetsParentID(t *testing.T) {
	database, user := setupUsers(t)

	if err := user.Posts().Save(newPost(user, "Saved")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := postOwner(t, database, "Saved"); got != 2 {
		t.Errorf("Save: expected user_id=2, got %d", got)
	}

	posts := model.User{}.DB(user.GetCtx()).Where("id", 2).With("Posts").First().Posts().Items()
	if len(posts) != 1 || posts[0].Title != "Saved" {
		t.Errorf("Save: expected Posts() to load the saved post, got %d posts", len(posts))
	}
}

func TestHasOneSaveSetsParentID(t *testing.T) {
	database, user := setupUsers(t)

	if err := user.Post().Save(newPost(user, "Only")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := postOwner(t, database, "Only"); got != 2 {
		t.Errorf("HasOne Save: expected user_id=2, got %d", got)
	}
}

func TestHasManyMorphSaveSetsParentIDAndType(t *testing.T) {
	database, user := setupUsers(t)

	comment := model.Comment{}.DB(user.GetCtx())
	comment.Body = "Hej"
	if err := user.Comments().Save(comment); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var id int64
	var kind, body string
	row := database.DB.QueryRow("SELECT commentable_id, commentable_type, body FROM comments")
	if err := row.Scan(&id, &kind, &body); err != nil {
		t.Fatal(err)
	}
	if id != 2 || kind != "user" || body != "Hej" {
		t.Errorf("Morph Save: expected (2, user, Hej), got (%d, %s, %s)", id, kind, body)
	}

	comments := model.User{}.DB(user.GetCtx()).Where("id", 2).With("Comments").First().Comments().Items()
	if len(comments) != 1 {
		t.Errorf("Morph Save: expected Comments() to load 1 comment, got %d", len(comments))
	}
}

func TestHasManyMorphAttachSetsParentIDAndType(t *testing.T) {
	database, user := setupUsers(t)

	// Attach calls data.DB(ctx) again, so the child keeps its own fields only
	// when its DB() copies the receiver, as in going_docs' concerns.md.
	if err := user.Comments().Attach(model.Comment{Body: "Vedhæftet"}.DB(user.GetCtx())); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	var id int64
	var kind, body string
	row := database.DB.QueryRow("SELECT commentable_id, commentable_type, body FROM comments")
	if err := row.Scan(&id, &kind, &body); err != nil {
		t.Fatal(err)
	}
	if id != 2 || kind != "user" || body != "Vedhæftet" {
		t.Errorf("Morph Attach: expected (2, user, Vedhæftet), got (%d, %s, %s)", id, kind, body)
	}

	// The documented read side: With("Comments") + Items().
	comments := model.User{}.DB(user.GetCtx()).With("Comments").Where("id", 2).First().Comments().Items()
	if len(comments) != 1 || comments[0].Body != "Vedhæftet" {
		t.Errorf("Morph Attach: expected With(\"Comments\") to load the comment, got %d", len(comments))
	}
}

func TestRelationSaveOnUnsavedParentFails(t *testing.T) {
	_, user := setupUsers(t)
	unsaved := model.User{}.DB(user.GetCtx())

	if err := unsaved.Posts().Attach(newPost(user, "Orphan")); err == nil {
		t.Error("HasMany Attach on an unsaved parent: expected an error")
	}
	if err := unsaved.Post().Save(newPost(user, "Orphan")); err == nil {
		t.Error("HasOne Save on an unsaved parent: expected an error")
	}
	if err := unsaved.Comments().Save(model.Comment{}.DB(user.GetCtx())); err == nil {
		t.Error("HasManyMorph Save on an unsaved parent: expected an error")
	}
}
