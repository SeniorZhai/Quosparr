package l10n

func init() {
	tuiStrings["balanceUnsupported"] = [7]string{
		"API account: no compatible balance endpoint found",
		"API 账号：未找到兼容的余额查询接口",
		"API 帳號：未找到相容的餘額查詢介面",
		"API 계정: 호환되는 잔액 조회 API를 찾을 수 없습니다",
		"API アカウント：対応する残高照会 API が見つかりません",
		"Аккаунт API: совместимый API баланса не найден",
		"Compte API : aucun point de terminaison de solde compatible trouvé",
	}
	tuiStrings["configError"] = [7]string{
		"Could not read or save TUI configuration. Check ~/.quosparr/tui and its permissions.",
		"无法读取或保存 TUI 配置，请检查 ~/.quosparr/tui 及其权限。",
		"無法讀取或儲存 TUI 設定，請檢查 ~/.quosparr/tui 及其權限。",
		"TUI 설정을 읽거나 저장할 수 없습니다. ~/.quosparr/tui 및 권한을 확인하세요.",
		"TUI 設定を読み取りまたは保存できません。~/.quosparr/tui とアクセス権を確認してください。",
		"Не удалось прочитать или сохранить настройки TUI. Проверьте ~/.quosparr/tui и права доступа.",
		"Impossible de lire ou d’enregistrer la configuration TUI. Vérifiez ~/.quosparr/tui et ses permissions.",
	}
}
