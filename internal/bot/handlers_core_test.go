package bot

import (
	"context"
	"errors"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/yelinaung/expense-bot/internal/bot/mocks"
	"gitlab.com/yelinaung/expense-bot/internal/gemini"
	appmodels "gitlab.com/yelinaung/expense-bot/internal/models"
	"google.golang.org/genai"
)

const nilMessageReturnsEarlyCore = "nil message returns early"

func TestHandleAddCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleAddCore(ctx, mockBot, update)

		require.Equal(t, 0, mockBot.SentMessageCount(), "no message should be sent for nil message")
	})

	t.Run("valid expense is saved", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(100001)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "testuser",
			FirstName: "Test",
		})
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
				Text: "/add 5.50 Coffee",
			},
		}

		b.handleAddCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, "$5.50 SGD")
		require.Contains(t, msg.Text, "Coffee")
	})

	t.Run("invalid format sends error message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(100002)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "testuser2",
			FirstName: "Test2",
		})
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
				Text: "/add invalid",
			},
		}

		b.handleAddCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Invalid format")
	})

	t.Run("expense with category is saved correctly", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(100003)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "testuser3",
			FirstName: "Test3",
		})
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
				Text: "/add 12.99 Lunch Food",
			},
		}

		b.handleAddCore(ctx, mockBot, update)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, "$12.99 SGD")
		require.Contains(t, msg.Text, "Lunch")
		require.Contains(t, msg.Text, "Food")
	})

	t.Run("send message error is handled gracefully", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		mockBot.SendMessageError = errors.New("telegram api error")
		userID := int64(100004)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "testuser4",
			FirstName: "Test4",
		})
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
				Text: "/add 5.50 Coffee",
			},
		}

		b.handleAddCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})
}

func TestHandleStartCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleStartCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("sends welcome message with name", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: 1, FirstName: "Alice"},
			},
		}
		b.handleStartCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Welcome")
		require.Contains(t, msg.Text, "Alice")
	})

	t.Run("sends welcome message without name", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: nil,
			},
		}
		b.handleStartCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Welcome")
	})
}

func TestHandleHelpCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleHelpCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("sends help message with commands", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: 1},
			},
		}
		b.handleHelpCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "/add")
		require.Contains(t, msg.Text, "/list")
		require.Contains(t, msg.Text, "/today")
		require.Contains(t, msg.Text, "/week")
		require.Contains(t, msg.Text, "/categories")
		require.Contains(t, msg.Text, "/edit")
		require.Contains(t, msg.Text, "/delete")
		require.Contains(t, msg.Text, "/addcategory")
		require.Contains(t, msg.Text, "/currency")
		require.Contains(t, msg.Text, "/setcurrency")
	})
}

func TestHandleCategoriesCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleCategoriesCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("lists all categories", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: 1},
			},
		}
		b.handleCategoriesCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Categories")
		require.Contains(t, msg.Text, "Food")
	})
}

func TestHandleListCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(300001)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "listuser",
		FirstName: "List",
	})
	require.NoError(t, err)

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleListCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("shows empty list message", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
			},
		}
		b.handleListCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Recent Expenses")
		require.Contains(t, msg.Text, "No expenses found")
	})

	t.Run("shows expenses when present", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("15.00"),
			Currency:    "SGD",
			Description: "Test Item",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
			},
		}
		b.handleListCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Recent Expenses")
		require.Contains(t, msg.Text, "$15.00")
		require.Contains(t, msg.Text, "Test Item")
	})
}

func TestHandleTodayCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(300002)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "todayuser",
		FirstName: "Today",
	})
	require.NoError(t, err)

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleTodayCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("shows today expenses with total", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("20.00"),
			Currency:    "SGD",
			Description: "Today Item",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
			},
		}
		b.handleTodayCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Today's Expenses")
		require.Contains(t, msg.Text, "Total:")
		require.Contains(t, msg.Text, "$20.00")
	})
}

func TestHandleWeekCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()
	userID := int64(300003)

	err := b.userRepo.UpsertUser(ctx, &appmodels.User{
		ID:        userID,
		Username:  "weekuser",
		FirstName: "Week",
	})
	require.NoError(t, err)

	t.Run(nilMessageReturnsEarlyCore, func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		update := &models.Update{Message: nil}
		b.handleWeekCore(ctx, mockBot, update)
		require.Equal(t, 0, mockBot.SentMessageCount())
	})

	t.Run("shows week expenses with total", func(t *testing.T) {
		mockBot := mocks.NewMockBot()

		expense := &appmodels.Expense{
			UserID:      userID,
			Amount:      mustParseDecimal("30.00"),
			Currency:    "SGD",
			Description: "Week Item",
		}
		err := b.expenseRepo.Create(ctx, expense)
		require.NoError(t, err)

		update := &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: 12345},
				From: &models.User{ID: userID},
			},
		}
		b.handleWeekCore(ctx, mockBot, update)
		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "This Week's Expenses")
		require.Contains(t, msg.Text, "Total:")
		require.Contains(t, msg.Text, "$30.00")
	})
}

func TestSaveExpenseCore(t *testing.T) {
	pool := TestDB(t)
	b := setupTestBot(t, pool)
	ctx := context.Background()

	t.Run("expense without category is saved as uncategorized", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(200001)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "saveuser1",
			FirstName: "Save1",
		})
		require.NoError(t, err)

		parsed := &ParsedExpense{
			Amount:      mustParseDecimal("10.00"),
			Description: "Test expense",
		}

		b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, "$10.00 SGD")
		require.Contains(t, msg.Text, "Uncategorized")
	})

	t.Run("expense with matching category sets category", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(200002)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "saveuser2",
			FirstName: "Save2",
		})
		require.NoError(t, err)

		categories, err := b.categoryRepo.GetAll(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, categories)

		parsed := &ParsedExpense{
			Amount:       mustParseDecimal("25.00"),
			Description:  "Groceries",
			CategoryName: categories[0].Name,
		}

		b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, categories)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, categories[0].Name)
	})

	t.Run("expense without matched category defaults to Others when available", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(200004)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "saveuser4",
			FirstName: "Save4",
		})
		require.NoError(t, err)

		categories, err := b.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		parsed := &ParsedExpense{
			Amount:      mustParseDecimal("18.00"),
			Description: "valentine roses",
		}

		b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, categories)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.Contains(t, msg.Text, "Others")
	})

	t.Run("ai can suggest and create a new category when unmatched", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(200005)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "saveuser5",
			FirstName: "Save5",
		})
		require.NoError(t, err)

		b.geminiClient = gemini.NewClientWithGenerator(&botTestGenerator{
			response: makeBotCategorySuggestionResponse(`{
				"category": "",
				"confidence": 0.95,
				"reasoning": "Recurring software subscription",
				"matched": false,
				"new_category_name": "AI Subscriptions"
			}`),
		})

		categories, err := b.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		parsed := &ParsedExpense{
			Amount:      mustParseDecimal("19.99"),
			Description: "ChatGPT monthly",
		}

		b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, categories)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "AI Subscriptions")

		createdCat, err := b.categoryRepo.GetByName(ctx, "AI Subscriptions")
		require.NoError(t, err)
		require.NotNil(t, createdCat)
	})

	t.Run("invalid ai new category suggestion falls back to Others", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(200006)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "saveuser6",
			FirstName: "Save6",
		})
		require.NoError(t, err)

		b.geminiClient = gemini.NewClientWithGenerator(&botTestGenerator{
			response: makeBotCategorySuggestionResponse(`{
				"category": "",
				"confidence": 0.95,
				"reasoning": "Bad suggestion",
				"matched": false,
				"new_category_name": "\u0000"
			}`),
		})

		categories, err := b.categoryRepo.GetAll(ctx)
		require.NoError(t, err)

		parsed := &ParsedExpense{
			Amount:      mustParseDecimal("8.00"),
			Description: "unknown item",
		}

		b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, categories)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Others")
	})

	t.Run("empty description is handled", func(t *testing.T) {
		mockBot := mocks.NewMockBot()
		userID := int64(200003)

		err := b.userRepo.UpsertUser(ctx, &appmodels.User{
			ID:        userID,
			Username:  "saveuser3",
			FirstName: "Save3",
		})
		require.NoError(t, err)

		parsed := &ParsedExpense{
			Amount:      mustParseDecimal("5.00"),
			Description: "",
		}

		b.saveExpenseCore(ctx, mockBot, 12345, userID, parsed, nil)

		require.Equal(t, 1, mockBot.SentMessageCount())
		msg := mockBot.LastSentMessage()
		require.Contains(t, msg.Text, "Expense Added")
		require.NotContains(t, msg.Text, "📝")
	})
}

type botTestGenerator struct {
	response *genai.GenerateContentResponse
	err      error
}

func (m *botTestGenerator) GenerateContent(
	_ context.Context,
	_ string,
	_ []*genai.Content,
	_ *genai.GenerateContentConfig,
) (*genai.GenerateContentResponse, error) {
	return m.response, m.err
}

func makeBotCategorySuggestionResponse(jsonText string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: jsonText},
					},
				},
			},
		},
	}
}
