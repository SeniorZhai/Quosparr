package app

const (
	xCostOverviewToday   textKey = "costOverviewToday"
	xCostOverviewWeek    textKey = "costOverviewWeek"
	xCostOverviewMonth   textKey = "costOverviewMonth"
	xCostOverviewInfo    textKey = "costOverviewInfo"
	xCostOverviewPartial textKey = "costOverviewPartial"
	xCostOverviewFailed  textKey = "costOverviewFailed"
)

func init() {
	extraTexts[xCostOverviewToday] = [7]string{"Today", "今天", "今天", "오늘", "今日", "Сегодня", "Aujourd’hui"}
	extraTexts[xCostOverviewWeek] = [7]string{"7 days", "7 天", "7 天", "7일", "7 日間", "7 дней", "7 jours"}
	extraTexts[xCostOverviewMonth] = [7]string{"30 days", "30 天", "30 天", "30일", "30 日間", "30 дней", "30 jours"}
	extraTexts[xCostOverviewInfo] = [7]string{
		"API-equivalent cost of all local Codex, Claude Code and Grok sessions, not a bill. Local calendar days, including today. Providers without local token records are not included.",
		"所有本机 Codex、Claude Code 和 Grok 会话的 API 等价费用，并非实际账单。按本地日期统计，包含今天；没有本地 Token 记录的供应商不计入。",
		"所有本機 Codex、Claude Code 和 Grok 工作階段的 API 等價費用，並非實際帳單。按本地日期統計，包含今天；沒有本地 Token 記錄的供應商不計入。",
		"모든 로컬 Codex, Claude Code 및 Grok 세션의 API 환산 비용이며 실제 청구액은 아닙니다. 오늘을 포함한 현지 날짜 기준이며 로컬 토큰 기록이 없는 공급자는 제외됩니다.",
		"ローカルの Codex、Claude Code、Grok 全セッションの API 換算費用です。実際の請求額ではありません。今日を含む現地日付で集計し、ローカルトークン記録のないプロバイダーは含みません。",
		"Эквивалентная стоимость API всех локальных сеансов Codex, Claude Code и Grok, а не счёт. Календарные дни по местному времени, включая сегодня. Сервисы без локальных записей токенов не учитываются.",
		"Coût équivalent API de toutes les sessions locales Codex, Claude Code et Grok, et non une facture. Jours calendaires locaux, aujourd’hui inclus. Les services sans relevés locaux de jetons sont exclus.",
	}
	extraTexts[xCostOverviewPartial] = [7]string{"Partial total: some usage or model prices are unavailable.", "部分合计：部分用量或模型价格缺失。", "部分合計：部分用量或模型價格缺失。", "일부 합계: 일부 사용량 또는 모델 가격이 없습니다.", "一部の合計：使用量またはモデル価格が不足しています。", "Неполная сумма: отсутствуют данные использования или цены моделей.", "Total partiel : certaines données d’usage ou certains tarifs manquent."}
	extraTexts[xCostOverviewFailed] = [7]string{"Refresh failed", "刷新失败", "重新整理失敗", "새로고침 실패", "更新に失敗", "Ошибка обновления", "Échec de l’actualisation"}
}
