package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
)

const callbackIDHandlers = "callback123"

func TestHandleEditCallbackCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "editcallbackuser",
		FirstName: "Edit",
	})
	require.NoError(t, err)

	t.Run("nil callback query returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{CallbackQuery: nil}
		b.handleEditCallbackCore(ctx, mockBot, update)
		require.Empty(t, mockBot.AnsweredCallbacks)
	})

	t.Run("invalid data format returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: "edit",
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleEditCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Empty(t, mockBot.EditedMessages)
	})

	t.Run("edit amount action shows prompt", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("50.00"),
			Currency:    "SGD",
			Description: "Test Expense",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: fmt.Sprintf("edit_amount_%d", expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleEditCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Edit Amount")
	})

	t.Run("edit category action shows selection", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("60.00"),
			Currency:    "SGD",
			Description: "Category Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: fmt.Sprintf("edit_category_%d", expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleEditCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Select Category")
	})

	t.Run("edit_expense callback is delegated to inline action handler", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("80.00"),
			Currency:    "SGD",
			Description: "Inline Edit",
			Status:      appmodels.ExpenseStatusConfirmed,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   "callback-inline-edit",
				From: models.User{ID: userID},
				Data: fmt.Sprintf("edit_expense_%d", expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   101,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}

		b.handleEditCallbackCore(ctx, mockBot, update)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Edit Expense #")
	})

	t.Run("user mismatch returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		otherUserID := userID + 100

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        otherUserID,
			Username:  "otheredituser",
			FirstName: "Other",
		})
		require.NoError(t, err)

		expense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("70.00"),
			Currency:    "SGD",
			Description: "Other User",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: fmt.Sprintf("edit_amount_%d", expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleEditCallbackCore(ctx, mockBot, update)
		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Empty(t, mockBot.EditedMessages)
	})
}

func TestPromptEditAmountCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500002)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "promptamountuser",
		FirstName: "Prompt",
	})
	require.NoError(t, err)

	t.Run("stores pending edit and shows prompt", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("25.50"),
			Currency:    "SGD",
			Description: "Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.promptEditAmountCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Edit Amount")
		require.Contains(t, mockBot.EditedMessages[0].Text, "$25.50 SGD")

		b.pendingEditsMu.RLock()
		pending, exists := b.pendingEdits[12345]
		b.pendingEditsMu.RUnlock()
		require.True(t, exists)
		require.Equal(t, expense.ID, pending.ExpenseID)
		require.Equal(t, "amount", pending.EditType)
	})
}

func TestHandlePendingEditCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500003)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "pendinguser",
		FirstName: "Pending",
	})
	require.NoError(t, err)

	t.Run("nil message returns false", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		result := b.handlePendingEditCore(ctx, mockBot, update)
		require.False(t, result)
	})

	t.Run("empty text returns false", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
				Text: "",
			},
		}
		result := b.handlePendingEditCore(ctx, mockBot, update)
		require.False(t, result)
	})

	t.Run("no pending edit returns false", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 99999},
				From: &models.User{ID: userID},
				Text: "25.00",
			},
		}
		result := b.handlePendingEditCore(ctx, mockBot, update)
		require.False(t, result)
	})

	t.Run("processes amount edit when pending", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("10.00"),
			Currency:    "SGD",
			Description: "Original",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		chatID := int64(500003)
		b.pendingEditsMu.Lock()
		b.pendingEdits[chatID] = &pendingEdit{
			ExpenseID: expense.ID,
			EditType:  "amount",
			MessageID: 100,
		}
		b.pendingEditsMu.Unlock()

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: chatID},
				From: &models.User{ID: userID},
				Text: "35.00",
			},
		}

		result := b.handlePendingEditCore(ctx, mockBot, update)
		require.True(t, result)

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "35", updated.Amount.StringFixed(0))
	})
}

func TestProcessAmountEditCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500004)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "amountuser",
		FirstName: "Amount",
	})
	require.NoError(t, err)

	t.Run("invalid amount shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		pending := &pendingEdit{ExpenseID: 1, EditType: "amount", MessageID: 100}

		b.pendingEditsMu.Lock()
		b.pendingEdits[12345] = pending
		b.pendingEditsMu.Unlock()

		result := b.processAmountEditCore(ctx, mockBot, 12345, userID, pending, "invalid")
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Invalid amount")
	})

	t.Run("expense not found shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		pending := &pendingEdit{ExpenseID: 99999, EditType: "amount", MessageID: 100}

		result := b.processAmountEditCore(ctx, mockBot, 12345, userID, pending, "25.00")
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Expense not found")
	})

	t.Run("user mismatch returns silently", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		otherUserID := userID + 100

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        otherUserID,
			Username:  "otheramountuser",
			FirstName: "Other",
		})
		require.NoError(t, err)

		expense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("15.00"),
			Currency:    "SGD",
			Description: "Other",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		pending := &pendingEdit{ExpenseID: expense.ID, EditType: "amount", MessageID: 100}
		result := b.processAmountEditCore(ctx, mockBot, 12345, userID, pending, "25.00")
		require.True(t, result)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("valid amount updates expense", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("20.00"),
			Currency:    "SGD",
			Description: "To Update",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		pending := &pendingEdit{ExpenseID: expense.ID, EditType: "amount", MessageID: 100}
		result := b.processAmountEditCore(ctx, mockBot, 12345, userID, pending, "$45.50")
		require.True(t, result)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Amount Updated")
		require.Contains(t, mockBot.EditedMessages[0].Text, "$45.50 SGD")

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "45.50", updated.Amount.StringFixed(2))
	})
}

func TestHandleCancelEditCallbackCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500005)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "canceluser",
		FirstName: "Cancel",
	})
	require.NoError(t, err)

	t.Run("nil callback query returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{CallbackQuery: nil}
		b.handleCancelEditCallbackCore(ctx, mockBot, update)
		require.Empty(t, mockBot.AnsweredCallbacks)
	})

	t.Run("clears pending edit and returns to edit menu", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("30.00"),
			Currency:    "SGD",
			Description: "Cancel Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		chatID := int64(12345)
		b.pendingEditsMu.Lock()
		b.pendingEdits[chatID] = &pendingEdit{
			ExpenseID: expense.ID,
			EditType:  "amount",
			MessageID: 100,
		}
		b.pendingEditsMu.Unlock()

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: fmt.Sprintf("cancel_edit_%d", expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: chatID},
					},
				},
			},
		}
		b.handleCancelEditCallbackCore(ctx, mockBot, update)

		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Edit Expense")

		b.pendingEditsMu.RLock()
		_, exists := b.pendingEdits[chatID]
		b.pendingEditsMu.RUnlock()
		require.False(t, exists)
	})
}

func TestShowCategorySelectionCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500006)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "catseluser",
		FirstName: "CatSel",
	})
	require.NoError(t, err)

	t.Run("shows category selection keyboard", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("40.00"),
			Currency:    "SGD",
			Description: "Category Select",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		b.showCategorySelectionCore(ctx, mockBot, 12345, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Select Category")
		require.NotNil(t, mockBot.EditedMessages[0].ReplyMarkup)
	})
}

func TestHandleSetCategoryCallbackCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500007)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "setcatuser",
		FirstName: "SetCat",
	})
	require.NoError(t, err)

	t.Run("nil callback query returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{CallbackQuery: nil}
		b.handleSetCategoryCallbackCore(ctx, mockBot, update)
		require.Empty(t, mockBot.AnsweredCallbacks)
	})

	t.Run("sets category and shows confirmation", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		categories, err := b.categoryRepo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("55.00"),
			Currency:    "SGD",
			Description: "Set Category",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: fmt.Sprintf("set_category_%d_%d", expense.ID, categories[0].ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleSetCategoryCallbackCore(ctx, mockBot, update)

		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Receipt Updated")
		require.Contains(t, mockBot.EditedMessages[0].Text, categories[0].Name)

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.NotNil(t, updated.CategoryID)
		require.Equal(t, categories[0].ID, *updated.CategoryID)
	})
}

func TestProcessCategoryCreateCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500008)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "createcatuser",
		FirstName: "CreateCat",
	})
	require.NoError(t, err)

	t.Run("empty name shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		pending := &pendingEdit{ExpenseID: 1, EditType: "category", MessageID: 100}

		result := b.processCategoryCreateCore(ctx, mockBot, 12345, userID, pending, "   ")
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "cannot be empty")
	})

	t.Run("name too long shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		pending := &pendingEdit{ExpenseID: 1, EditType: "category", MessageID: 100}

		longName := "This category name is way too long and exceeds the fifty character limit"
		result := b.processCategoryCreateCore(ctx, mockBot, 12345, userID, pending, longName)
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "too long")
	})

	t.Run("expense not found shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		pending := &pendingEdit{ExpenseID: 99999, EditType: "category", MessageID: 100}

		result := b.processCategoryCreateCore(ctx, mockBot, 12345, userID, pending, "NewCat")
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Expense not found")
	})

	t.Run("creates category and assigns to expense", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("65.00"),
			Currency:    "SGD",
			Description: "Create Cat Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		pending := &pendingEdit{ExpenseID: expense.ID, EditType: "category", MessageID: 100}
		result := b.processCategoryCreateCore(ctx, mockBot, 12345, userID, pending, "MyNewCategory")
		require.True(t, result)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Category Created")
		require.Contains(t, mockBot.EditedMessages[0].Text, "MyNewCategory")

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.NotNil(t, updated.CategoryID)
	})
}

func TestHandleCreateCategoryCallbackCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500009)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "createcallbackuser",
		FirstName: "CreateCallback",
	})
	require.NoError(t, err)

	t.Run("nil callback query returns early", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{CallbackQuery: nil}
		b.handleCreateCategoryCallbackCore(ctx, mockBot, update)
		require.Empty(t, mockBot.AnsweredCallbacks)
	})

	t.Run("shows create category prompt", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("75.00"),
			Currency:    "SGD",
			Description: "Create Callback",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			CallbackQuery: &models.CallbackQuery{
				ID:   callbackIDHandlers,
				From: models.User{ID: userID},
				Data: fmt.Sprintf("create_category_%d", expense.ID),
				Message: models.MaybeInaccessibleMessage{
					Message: &models.Message{
						ID:   100,
						Chat: models.Chat{ID: 12345},
					},
				},
			},
		}
		b.handleCreateCategoryCallbackCore(ctx, mockBot, update)

		require.Len(t, mockBot.AnsweredCallbacks, 1)
		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Create New Category")
	})
}

func TestPromptEditMerchantCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500020)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "merchantpromptuser",
		FirstName: "MerchantPrompt",
	})
	require.NoError(t, err)

	t.Run("stores pending edit and shows prompt", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("15.00"),
			Currency:    "SGD",
			Description: "Old Merchant",
			Merchant:    "Old Merchant",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		chatID := int64(22222)
		b.promptEditMerchantCore(ctx, mockBot, chatID, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Edit Merchant")
		require.Contains(t, mockBot.EditedMessages[0].Text, "Old Merchant")

		b.pendingEditsMu.RLock()
		pending, exists := b.pendingEdits[chatID]
		b.pendingEditsMu.RUnlock()
		require.True(t, exists)
		require.Equal(t, expense.ID, pending.ExpenseID)
		require.Equal(t, "merchant", pending.EditType)
	})
}

func TestProcessMerchantEditCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500021)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "merchantedituser",
		FirstName: "MerchantEdit",
	})
	require.NoError(t, err)

	t.Run("empty merchant shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		chatID := int64(33333)
		pending := &pendingEdit{ExpenseID: 1, EditType: "merchant", MessageID: 100}

		b.pendingEditsMu.Lock()
		b.pendingEdits[chatID] = pending
		b.pendingEditsMu.Unlock()

		result := b.processMerchantEditCore(ctx, mockBot, chatID, userID, pending, "   ")
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "cannot be empty")
	})

	t.Run("expense not found shows error", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		pending := &pendingEdit{ExpenseID: 99999, EditType: "merchant", MessageID: 100}

		result := b.processMerchantEditCore(ctx, mockBot, 33334, userID, pending, "New Merchant")
		require.True(t, result)
		require.Equal(t, 1, mockBot.SentMessageCount())
		require.Contains(t, mockBot.LastSentMessage().Text, "Expense not found")
	})

	t.Run("user mismatch returns silently", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		otherUserID := userID + 100

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        otherUserID,
			Username:  "othermerchantuser",
			FirstName: "Other",
		})
		require.NoError(t, err)

		expense := &appmodels.Expense{
			UserID:      otherUserID,
			Amount:      mustParseDecimal("10.00"),
			Currency:    "SGD",
			Description: "Other Merchant",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err = b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		pending := &pendingEdit{ExpenseID: expense.ID, EditType: "merchant", MessageID: 100}
		result := b.processMerchantEditCore(ctx, mockBot, 33335, userID, pending, "New Merchant")
		require.True(t, result)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("valid merchant updates expense", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("20.00"),
			Currency:    "SGD",
			Description: "Old Name",
			Merchant:    "Old Name",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		pending := &pendingEdit{ExpenseID: expense.ID, EditType: "merchant", MessageID: 100}
		result := b.processMerchantEditCore(ctx, mockBot, 33336, userID, pending, "New Restaurant")
		require.True(t, result)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Merchant Updated")
		require.Contains(t, mockBot.EditedMessages[0].Text, "New Restaurant")

		updated, err := b.expenseRepo.GetByID(ctx, expense.ID)
		require.NoError(t, err)
		require.Equal(t, "New Restaurant", updated.Merchant)
		require.Equal(t, "New Restaurant", updated.Description)
	})

	t.Run("clears pending edit on success", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		chatID := int64(33337)

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("15.00"),
			Currency:    "SGD",
			Description: "Clear Test",
			Merchant:    "Clear Test",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		pending := &pendingEdit{ExpenseID: expense.ID, EditType: "merchant", MessageID: 100}
		b.pendingEditsMu.Lock()
		b.pendingEdits[chatID] = pending
		b.pendingEditsMu.Unlock()

		result := b.processMerchantEditCore(ctx, mockBot, chatID, userID, pending, "Updated Merchant")
		require.True(t, result)

		b.pendingEditsMu.RLock()
		_, exists := b.pendingEdits[chatID]
		b.pendingEditsMu.RUnlock()
		require.False(t, exists)
	})
}

func TestPromptCreateCategoryCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(500010)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "promptcatuser",
		FirstName: "PromptCat",
	})
	require.NoError(t, err)

	t.Run("stores pending edit and shows prompt", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("85.00"),
			Currency:    "SGD",
			Description: "Prompt Cat",
			Status:      appmodels.ExpenseStatusDraft,
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		chatID := int64(12345)
		b.promptCreateCategoryCore(ctx, mockBot, chatID, 100, expense)

		require.Len(t, mockBot.EditedMessages, 1)
		require.Contains(t, mockBot.EditedMessages[0].Text, "Create New Category")

		b.pendingEditsMu.RLock()
		pending, exists := b.pendingEdits[chatID]
		b.pendingEditsMu.RUnlock()
		require.True(t, exists)
		require.Equal(t, expense.ID, pending.ExpenseID)
		require.Equal(t, "category", pending.EditType)
	})
}
