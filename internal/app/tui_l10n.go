package app

const (
	xTUIWaiting        textKey = "tuiWaiting"
	xTUISnapshotStale  textKey = "tuiSnapshotStale"
	xTUISnapshotFailed textKey = "tuiSnapshotFailed"
	xTUIStartFailed    textKey = "tuiStartFailed"
	xTUILegacyRunning  textKey = "tuiLegacyRunning"
)

func init() {
	for key, texts := range map[textKey][7]string{
		xTUIWaiting:        {"Waiting for a snapshot from the running Quosparr; update or restart that app if needed", "等待运行中的 Quosparr 提供快照；如一直未显示，请更新或重启该应用", "等待執行中的 Quosparr 提供快照；如一直未顯示，請更新或重新啟動該應用程式", "실행 중인 Quosparr의 스냅샷을 기다리는 중입니다. 필요하면 앱을 업데이트하거나 다시 시작하세요", "実行中の Quosparr のスナップショットを待機中です。必要に応じて更新または再起動してください", "Ожидание снимка запущенного Quosparr; при необходимости обновите или перезапустите приложение", "En attente d’un instantané de Quosparr ; mettez à jour ou redémarrez l’application si nécessaire"},
		xTUISnapshotStale:  {"Snapshot is stale; the source app may have stopped", "快照已过期，来源应用可能已停止", "快照已過期，來源應用程式可能已停止", "스냅샷이 오래되었습니다. 원본 앱이 중지되었을 수 있습니다", "スナップショットが古くなっています。元のアプリが停止した可能性があります", "Снимок устарел; приложение-источник могло остановиться", "L’instantané est périmé ; l’application source est peut-être arrêtée"},
		xTUISnapshotFailed: {"Could not read the running app’s snapshot", "无法读取运行中应用的快照", "無法讀取執行中應用程式的快照", "실행 중인 앱의 스냅샷을 읽을 수 없습니다", "実行中のアプリのスナップショットを読み込めません", "Не удалось прочитать снимок запущенного приложения", "Impossible de lire l’instantané de l’application"},
		xTUIStartFailed:    {"Could not start TUI; check the Quosparr data directory and instance lock", "无法启动 TUI，请检查 Quosparr 数据目录和实例锁", "無法啟動 TUI，請檢查 Quosparr 資料目錄與執行個體鎖定", "TUI를 시작할 수 없습니다. Quosparr 데이터 폴더와 인스턴스 잠금을 확인하세요", "TUI を起動できません。Quosparr のデータフォルダーとインスタンスロックを確認してください", "Не удалось запустить TUI; проверьте каталог данных и блокировку Quosparr", "Impossible de lancer le TUI ; vérifiez le dossier de données et le verrou de Quosparr"},
		xTUILegacyRunning:  {"Quit the previous app before starting Quosparr TUI", "请先退出旧版应用，再启动 Quosparr TUI", "請先結束舊版應用程式，再啟動 Quosparr TUI", "이전 앱을 종료한 후 Quosparr TUI를 시작하세요", "旧アプリを終了してから Quosparr TUI を起動してください", "Закройте прежнее приложение перед запуском Quosparr TUI", "Quittez l’ancienne application avant de lancer Quosparr TUI"},
	} {
		extraTexts[key] = texts
	}
}
