package repo

import (
	"testing"
	"time"
)

func createTestUser(t *testing.T, repo *UserRepo, username string) string {
	t.Helper()
	user, err := repo.Create(username, "hashed-pw", "member")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return user.ID
}

func TestConversationCreateAndGetByID(t *testing.T) {
	database := setupTestDB(t)
	userRepo := NewUserRepo(database)
	convRepo := NewConversationRepo(database)

	userID := createTestUser(t, userRepo, "alice")

	conv, err := convRepo.Create(userID, "My First Conversation")
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID == "" {
		t.Error("expected non-empty ID")
	}
	if conv.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, conv.UserID)
	}
	if conv.Title != "My First Conversation" {
		t.Errorf("expected title 'My First Conversation', got %s", conv.Title)
	}

	got, err := convRepo.GetByID(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != conv.ID {
		t.Error("IDs don't match")
	}
	if got.Title != conv.Title {
		t.Errorf("titles don't match: %s vs %s", got.Title, conv.Title)
	}
}

func TestConversationListByUser(t *testing.T) {
	database := setupTestDB(t)
	userRepo := NewUserRepo(database)
	convRepo := NewConversationRepo(database)

	aliceID := createTestUser(t, userRepo, "alice")
	bobID := createTestUser(t, userRepo, "bob")

	// Create conversations for alice with slight time separation
	conv1, err := convRepo.Create(aliceID, "Alice Convo 1")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	conv2, err := convRepo.Create(aliceID, "Alice Convo 2")
	if err != nil {
		t.Fatal(err)
	}

	// Create one for bob
	_, err = convRepo.Create(bobID, "Bob Convo")
	if err != nil {
		t.Fatal(err)
	}

	// ListByUser should return only alice's conversations
	convs, err := convRepo.ListByUser(aliceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected 2 conversations for alice, got %d", len(convs))
	}

	// Should be ordered by updated_at DESC, so conv2 first
	if convs[0].ID != conv2.ID {
		t.Errorf("expected conv2 first (newest), got %s", convs[0].ID)
	}
	if convs[1].ID != conv1.ID {
		t.Errorf("expected conv1 second (oldest), got %s", convs[1].ID)
	}

	// Bob's list should have only 1
	bobConvs, err := convRepo.ListByUser(bobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bobConvs) != 1 {
		t.Errorf("expected 1 conversation for bob, got %d", len(bobConvs))
	}
}

func TestConversationUpdateTitle(t *testing.T) {
	database := setupTestDB(t)
	userRepo := NewUserRepo(database)
	convRepo := NewConversationRepo(database)

	userID := createTestUser(t, userRepo, "alice")
	conv, err := convRepo.Create(userID, "Old Title")
	if err != nil {
		t.Fatal(err)
	}

	err = convRepo.UpdateTitle(conv.ID, "New Title")
	if err != nil {
		t.Fatal(err)
	}

	got, err := convRepo.GetByID(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "New Title" {
		t.Errorf("expected 'New Title', got %s", got.Title)
	}
}

func TestConversationDeleteCascadesMessages(t *testing.T) {
	database := setupTestDB(t)
	userRepo := NewUserRepo(database)
	convRepo := NewConversationRepo(database)

	userID := createTestUser(t, userRepo, "alice")
	conv, err := convRepo.Create(userID, "To Be Deleted")
	if err != nil {
		t.Fatal(err)
	}

	// Add a message
	msg := &MessageRecord{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        `[{"type":"text","text":"hello"}]`,
	}
	if err := convRepo.AddMessage(msg); err != nil {
		t.Fatal(err)
	}

	// Verify message exists
	msgs, err := convRepo.GetMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message before delete, got %d", len(msgs))
	}

	// Delete conversation
	err = convRepo.Delete(conv.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Conversation should be gone
	_, err = convRepo.GetByID(conv.ID)
	if err == nil {
		t.Error("deleted conversation should not be found")
	}

	// Messages should be cascade-deleted
	msgs, err = convRepo.GetMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", len(msgs))
	}
}

func TestConversationAddMessageAndGetMessages(t *testing.T) {
	database := setupTestDB(t)
	userRepo := NewUserRepo(database)
	convRepo := NewConversationRepo(database)

	userID := createTestUser(t, userRepo, "alice")
	conv, err := convRepo.Create(userID, "Chat")
	if err != nil {
		t.Fatal(err)
	}

	msgs := []MessageRecord{
		{
			ConversationID: conv.ID,
			Role:           "user",
			Content:        `[{"type":"text","text":"Hello!"}]`,
			Model:          "",
			Provider:       "",
		},
		{
			ConversationID: conv.ID,
			Role:           "assistant",
			Content:        `[{"type":"text","text":"Hi there!"}]`,
			Model:          "gpt-4",
			Provider:       "openai",
			TokensIn:       10,
			TokensOut:      5,
			Cost:           0.001,
			LatencyMs:      250,
		},
	}

	for i := range msgs {
		time.Sleep(2 * time.Millisecond)
		if err := convRepo.AddMessage(&msgs[i]); err != nil {
			t.Fatalf("add message %d: %v", i, err)
		}
	}

	got, err := convRepo.GetMessages(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}

	// Check order is ASC (user message first)
	if got[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %s", got[0].Role)
	}
	if got[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %s", got[1].Role)
	}

	// Verify assistant message fields
	asst := got[1]
	if asst.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", asst.Model)
	}
	if asst.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %s", asst.Provider)
	}
	if asst.TokensIn != 10 {
		t.Errorf("expected tokens_in 10, got %d", asst.TokensIn)
	}
	if asst.TokensOut != 5 {
		t.Errorf("expected tokens_out 5, got %d", asst.TokensOut)
	}
	if asst.LatencyMs != 250 {
		t.Errorf("expected latency_ms 250, got %d", asst.LatencyMs)
	}
}
