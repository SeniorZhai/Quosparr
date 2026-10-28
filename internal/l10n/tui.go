package l10n

// TUI returns localized terminal-mode copy.
func (l Localizer) TUI(key string) string {
	values, ok := tuiStrings[key]
	if !ok {
		return key
	}
	for i, lang := range Languages() {
		if lang == l.Language() {
			return values[i]
		}
	}
	return values[0]
}

var tuiStrings = map[string][7]string{
	"overview":         {"Quota overview", "额度总览", "額度總覽", "할당량 개요", "クォータ概要", "Обзор квот", "Vue des quotas"},
	"auto":             {"Auto refresh", "自动刷新", "自動重新整理", "자동 새로고침", "自動更新", "Автообновление", "Actualisation auto"},
	"refreshing":       {"Refreshing…", "正在刷新…", "正在重新整理…", "새로고침 중…", "更新中…", "Обновление…", "Actualisation…"},
	"next":             {"Next refresh", "下次刷新", "下次重新整理", "다음 새로고침", "次回の更新", "Следующее обновление", "Prochaine actualisation"},
	"last":             {"Last refresh", "上次刷新", "上次重新整理", "마지막 새로고침", "前回の更新", "Последнее обновление", "Dernière actualisation"},
	"sync":             {"Sync", "同步", "同步", "동기화", "同期", "Синхронизация", "Synchroniser"},
	"desktop":          {"Desktop sync", "桌面同步", "桌面同步", "데스크톱 동기화", "デスクトップ同期", "Синхронизация с приложением", "Synchro avec l’app"},
	"standalone":       {"Independent", "独立运行", "獨立執行", "독립 실행", "単独実行", "Автономный режим", "Autonome"},
	"live":             {"Live", "实时", "即時", "실시간", "リアルタイム", "В реальном времени", "En direct"},
	"balance":          {"Balance", "余额", "餘額", "잔액", "残高", "Баланс", "Solde"},
	"page":             {"Page", "页", "頁", "페이지", "ページ", "Страница", "Page"},
	"navigate":         {"Browse", "浏览", "瀏覽", "탐색", "切り替え", "Просмотр", "Parcourir"},
	"idle":             {"Waiting for usage", "等待用量", "等待用量", "사용량 대기 중", "使用状況を待機中", "Ожидание данных об использовании", "En attente d’utilisation"},
	"updated":          {"Synced", "已同步", "已同步", "동기화됨", "同期済み", "Синхронизировано", "Synchronisé"},
	"setup":            {"Setup", "配置", "設定", "설정", "設定", "Настройка", "Configuration"},
	"configure":        {"Configure", "配置", "設定", "설정", "設定", "Настроить", "Configurer"},
	"save":             {"Save", "保存", "儲存", "저장", "保存", "Сохранить", "Enregistrer"},
	"cancel":           {"Cancel", "取消", "取消", "취소", "キャンセル", "Отмена", "Annuler"},
	"providers":        {"Providers", "服务商", "供應商", "공급자", "プロバイダー", "Провайдеры", "Fournisseurs"},
	"interval":         {"Refresh interval", "刷新间隔", "重新整理間隔", "새로고침 간격", "更新間隔", "Интервал обновления", "Intervalle d’actualisation"},
	"language":         {"Language", "语言", "語言", "언어", "言語", "Язык", "Langue"},
	"setupIntro":       {"Choose providers and credentials, then confirm Save. Esc or Ctrl+C cancels.", "选择服务商与凭据来源，最后确认保存。Esc 或 Ctrl+C 可取消。", "選擇供應商與憑證來源，最後確認儲存。Esc 或 Ctrl+C 可取消。", "공급자와 자격 증명을 선택한 후 저장을 확인하세요. Esc 또는 Ctrl+C로 취소합니다.", "プロバイダーと認証情報を選択して保存を確認します。EscまたはCtrl+Cでキャンセル。", "Выберите провайдеров и учётные данные, затем подтвердите сохранение. Esc или Ctrl+C — отмена.", "Choisissez les fournisseurs et les identifiants, puis confirmez l’enregistrement. Esc ou Ctrl+C annule."},
	"invalidChoice":    {"Enter one of the listed choices.", "请输入有效选项。", "請輸入有效選項。", "유효한 항목을 입력하세요.", "有効な選択肢を入力してください。", "Введите допустимый вариант.", "Saisissez une option valide."},
	"intervalSeconds":  {"Refresh interval in seconds (30–86400)", "刷新间隔，单位秒（30–86400）", "重新整理間隔，單位秒（30–86400）", "새로고침 간격, 초 (30–86400)", "更新間隔、秒（30–86400）", "Интервал обновления, секунды (30–86400)", "Intervalle d’actualisation, secondes (30–86400)"},
	"sourceChoices":    {"1 Keep existing / use CLI · 2 Configure each · 3 Import from App", "1 保留现有 / 使用 CLI · 2 逐项配置 · 3 从 App 一键导入", "1 保留現有 / 使用 CLI · 2 逐項設定 · 3 從 App 一鍵匯入", "1 기존 유지 / CLI 사용 · 2 개별 설정 · 3 앱에서 가져오기", "1 現在の設定 / CLI · 2 個別設定 · 3 アプリからインポート", "1 Сохранить / CLI · 2 Настроить по отдельности · 3 Импорт из приложения", "1 Conserver / CLI · 2 Configurer séparément · 3 Importer depuis l’app"},
	"importMissing":    {"No usable current App account; existing setting kept", "未找到可用的 App 当前账号，保留原配置", "找不到可用的 App 目前帳號，保留原設定", "사용 가능한 앱의 현재 계정 없음; 기존 설정 유지", "利用可能なアプリの現在のアカウントなし。既存設定を保持", "Нет доступного текущего аккаунта приложения; настройка сохранена", "Aucun compte actuel utilisable dans l’app ; réglage conservé"},
	"importReady":      {"Import ready; saved only after confirmation", "已准备导入，确认后才保存", "已準備匯入，確認後才儲存", "가져오기 준비 완료; 확인 후 저장", "インポート準備完了。確認後に保存", "Импорт подготовлен; сохранение после подтверждения", "Import prêt ; enregistrement après confirmation"},
	"accountChoices":   {"1 Existing CLI login · 2 API Key · 3 Keep", "1 已有 CLI 登录 · 2 API Key · 3 保留现有", "1 現有 CLI 登入 · 2 API Key · 3 保留現有", "1 기존 CLI 로그인 · 2 API Key · 3 유지", "1 既存のCLIログイン · 2 API Key · 3 維持", "1 Вход CLI · 2 API Key · 3 Сохранить", "1 Connexion CLI · 2 API Key · 3 Conserver"},
	"keyChoices":       {"2 API Key · 3 Keep", "2 API Key · 3 保留现有", "2 API Key · 3 保留現有", "2 API Key · 3 유지", "2 API Key · 3 維持", "2 API Key · 3 Сохранить", "2 API Key · 3 Conserver"},
	"invalidURL":       {"Enter an HTTP or HTTPS site without embedded credentials.", "请输入不含内嵌凭据的 HTTP 或 HTTPS 站点。", "請輸入不含內嵌憑證的 HTTP 或 HTTPS 站點。", "자격 증명이 포함되지 않은 HTTP 또는 HTTPS 사이트를 입력하세요.", "認証情報を含まないHTTPまたはHTTPSのサイトを入力してください。", "Введите сайт HTTP или HTTPS без встроенных учётных данных.", "Saisissez un site HTTP ou HTTPS sans identifiants intégrés."},
	"kimiRegion":       {"Kimi region (cn / intl)", "Kimi 区域（cn / intl）", "Kimi 區域（cn / intl）", "Kimi 지역 (cn / intl)", "Kimiの地域（cn / intl）", "Регион Kimi (cn / intl)", "Région Kimi (cn / intl)"},
	"apiKeyHidden":     {"API key / token (hidden input)", "API Key / Token（隐藏输入）", "API Key / Token（隱藏輸入）", "API 키 / 토큰 (숨김 입력)", "APIキー / トークン（非表示入力）", "API-ключ / токен (скрытый ввод)", "Clé API / jeton (saisie masquée)"},
	"invalidKey":       {"Enter a nonempty key without spaces or control characters.", "请输入非空 Key，不得包含空白或控制字符。", "請輸入非空 Key，不得包含空白或控制字元。", "공백이나 제어 문자가 없는 키를 입력하세요.", "空白や制御文字を含まないキーを入力してください。", "Введите непустой ключ без пробелов и управляющих символов.", "Saisissez une clé non vide, sans espaces ni caractères de contrôle."},
	"cliSource":        {"Existing CLI login", "已有 CLI 登录", "現有 CLI 登入", "기존 CLI 로그인", "既存のCLIログイン", "Вход через CLI", "Connexion CLI existante"},
	"importSource":     {"Imported App credential", "App 凭据副本", "App 憑證副本", "가져온 앱 자격 증명", "アプリから取り込んだ認証情報", "Копия данных приложения", "Copie des identifiants de l’app"},
	"saveNotice":       {"Save writes TUI settings and new private credential copies. App and official CLI logins stay unchanged.", "保存会写入 TUI 配置与新的私有凭据副本，不改 App 和官方 CLI 登录。", "儲存會寫入 TUI 設定與新的私人憑證副本，不變更 App 與官方 CLI 登入。", "TUI 설정과 새 비공개 자격 증명 사본을 저장합니다. 앱과 공식 CLI 로그인은 변경하지 않습니다.", "TUI設定と新しい非公開の認証情報コピーを保存します。アプリと公式CLIのログインは変更しません。", "Сохраняются настройки TUI и новые приватные копии данных. Вход в приложение и официальный CLI не меняется.", "Enregistre les réglages TUI et de nouvelles copies privées des identifiants. Les connexions de l’app et des CLI officiels restent inchangées."},
	"savePrompt":       {"Save (1 yes / 0 cancel)", "保存（1 确认 / 0 取消）", "儲存（1 確認 / 0 取消）", "저장 (1 확인 / 0 취소)", "保存（1 確認 / 0 キャンセル）", "Сохранить (1 да / 0 отмена)", "Enregistrer (1 oui / 0 annuler)"},
	"loginCLI":         {"Sign in with %s", "请先运行 %s 登录", "請先執行 %s 登入", "%s로 로그인하세요", "%sでログインしてください", "Войдите с помощью %s", "Connectez-vous avec %s"},
	"expiredCLI":       {"CLI login expired; run %s again", "CLI 登录已过期，请重新运行 %s", "CLI 登入已過期，請重新執行 %s", "CLI 로그인이 만료되었습니다. %s를 다시 실행하세요", "CLIログインの有効期限切れ。%sを再実行してください", "Вход CLI истёк; снова выполните %s", "Connexion CLI expirée ; relancez %s"},
	"expiredImported":  {"Imported login expired; reimport or configure CLI login (%s)", "导入登录已过期；请重新导入或配置 CLI 登录（%s）", "匯入登入已過期；請重新匯入或設定 CLI 登入（%s）", "가져온 로그인이 만료됨; 다시 가져오거나 CLI 로그인 설정 (%s)", "取り込んだログインの有効期限切れ。再インポートまたはCLIログインを設定（%s）", "Импортированный вход истёк; повторите импорт или настройте вход CLI (%s)", "Connexion importée expirée ; réimportez ou configurez la connexion CLI (%s)"},
	"unconfigured":     {"Not configured", "尚未配置", "尚未設定", "설정되지 않음", "未設定", "Не настроено", "Non configuré"},
	"unavailable":      {"Unavailable", "暂不可用", "暫時無法使用", "사용할 수 없음", "利用不可", "Недоступно", "Indisponible"},
	"terminalRequired": {"TUI mode requires an interactive terminal for input and output.", "TUI 模式需要可交互的终端输入和输出。", "TUI 模式需要可互動的終端輸入和輸出。", "TUI 모드에는 대화형 터미널 입력과 출력이 필요합니다.", "TUIモードには対話型端末の入出力が必要です。", "Режиму TUI нужен интерактивный терминал для ввода и вывода.", "Le mode TUI nécessite un terminal interactif en entrée et en sortie."},
	"terminalError":    {"Terminal error", "终端错误", "終端錯誤", "터미널 오류", "端末エラー", "Ошибка терминала", "Erreur de terminal"},
	"remaining":        {"Remaining", "剩余", "剩餘", "남음", "残り", "Осталось", "Restant"},
	"reset":            {"Reset in", "重置倒计时", "重置倒數", "초기화까지", "リセットまで", "До сброса", "Réinitialisation dans"},
	"resize":           {"Enlarge the terminal to see more", "扩大终端以显示更多", "放大終端以顯示更多", "더 보려면 터미널을 확대하세요", "端末を広げると続きを表示", "Увеличьте терминал, чтобы увидеть больше", "Agrandissez le terminal pour en voir plus"},
}
