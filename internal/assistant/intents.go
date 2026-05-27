package assistant

type AssistantIntent string

const (
	IntentCalendarQuery       AssistantIntent = "calendar_query"
	IntentCalendarCreate      AssistantIntent = "calendar_create"
	IntentCalendarUpdate      AssistantIntent = "calendar_update"
	IntentUserProfileUpdate   AssistantIntent = "user_profile_update"
	IntentUserProfileQuestion AssistantIntent = "user_profile_question"
	IntentKnowledgeSave       AssistantIntent = "knowledge_save"
	IntentKnowledgeQuery      AssistantIntent = "knowledge_query"
	IntentAnchorFeedback      AssistantIntent = "anchor_feedback"
	IntentTodayDirection      AssistantIntent = "today_direction"
	IntentReplanDay           AssistantIntent = "replan_day"
	IntentDailyReview         AssistantIntent = "daily_review"
	IntentUnknown             AssistantIntent = "unknown"
)

func (i AssistantIntent) Valid() bool {
	switch i {
	case IntentCalendarQuery, IntentCalendarCreate, IntentCalendarUpdate,
		IntentUserProfileUpdate, IntentUserProfileQuestion,
		IntentKnowledgeSave, IntentKnowledgeQuery, IntentAnchorFeedback,
		IntentTodayDirection, IntentReplanDay, IntentDailyReview, IntentUnknown:
		return true
	default:
		return false
	}
}
