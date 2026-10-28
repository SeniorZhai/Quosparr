package app

const (
	xTUICommand            textKey = "tuiCommand"
	xTUICommandDescription textKey = "tuiCommandDescription"
	xTUIInstall            textKey = "tuiInstall"
	xTUIInstalled          textKey = "tuiInstalled"
	xTUIInstallFailed      textKey = "tuiInstallFailed"
	xTUIInstallSource      textKey = "tuiInstallSource"
	xTUIInstallConflict    textKey = "tuiInstallConflict"
)

func init() {
	for key, texts := range map[textKey][7]string{
		xTUICommand:            {"Terminal dashboard", "终端仪表盘", "終端儀表板", "터미널 대시보드", "ターミナルダッシュボード", "Панель в терминале", "Tableau de bord du terminal"},
		xTUICommandDescription: {"Independent quota monitor with automatic refresh and its own setup", "独立运行，自动刷新，首次启动引导配置", "獨立執行、自動重新整理，首次啟動引導設定", "독립 실행, 자동 새로고침, 첫 실행 시 설정 안내", "単独実行、自動更新、初回起動時に設定を案内", "Автономный монитор квот с автообновлением и настройкой при запуске", "Moniteur de quotas autonome avec actualisation automatique et configuration initiale"},
		xTUIInstall:            {"Install TUI", "安装 TUI", "安裝 TUI", "TUI 설치", "TUI をインストール", "Установить TUI", "Installer le TUI"},
		xTUIInstalled:          {"Installed to:\n%s\n\nRun quosparr in your terminal. If the command is not found, run the full path above. The first launch guides setup; use --setup to configure again. Shell settings were not changed.", "已安装到：\n%s\n\n在终端运行 quosparr。若找不到命令，请使用上面的完整路径。首次启动进入配置向导，之后可用 --setup 重新配置。未修改 Shell 设置。", "已安裝至：\n%s\n\n在終端執行 quosparr。若找不到命令，請使用上方完整路徑。首次啟動進入設定精靈，之後可用 --setup 重新設定。未修改 Shell 設定。", "설치 위치:\n%s\n\n터미널에서 quosparr를 실행하세요. 명령을 찾을 수 없으면 위 전체 경로를 사용하세요. 첫 실행 시 설정을 안내하며 --setup으로 다시 설정할 수 있습니다. 셸 설정은 변경되지 않았습니다.", "インストール先：\n%s\n\nターミナルで quosparr を実行してください。見つからない場合は上のフルパスを使用してください。初回は設定を案内します。再設定は --setup を使用します。シェル設定は変更していません。", "Установлено в:\n%s\n\nЗапустите quosparr в терминале. Если команда не найдена, используйте полный путь выше. При первом запуске откроется настройка; для повторной настройки используйте --setup. Настройки оболочки не изменены.", "Installé dans :\n%s\n\nExécutez quosparr dans le terminal. Si la commande est introuvable, utilisez le chemin complet ci-dessus. Le premier lancement guide la configuration ; --setup permet de la modifier. Les réglages du shell sont inchangés."},
		xTUIInstallFailed:      {"Could not install TUI. Check that your user command directory is writable.", "无法安装 TUI，请检查用户命令目录是否可写。", "無法安裝 TUI，請檢查使用者命令目錄是否可寫入。", "TUI를 설치할 수 없습니다. 사용자 명령 폴더의 쓰기 권한을 확인하세요.", "TUI をインストールできません。ユーザーコマンドフォルダーの書き込み権限を確認してください。", "Не удалось установить TUI. Проверьте доступ на запись в каталог команд пользователя.", "Impossible d’installer le TUI. Vérifiez les droits d’écriture du dossier de commandes utilisateur."},
		xTUIInstallSource:      {"This app does not contain a valid TUI for this system. Reinstall the latest Quosparr package.", "此应用未包含适用于本机的 TUI，请重新安装最新 Quosparr 安装包。", "此應用程式未包含適用於本機的 TUI，請重新安裝最新版 Quosparr。", "앱에 이 시스템용 TUI가 없습니다. 최신 Quosparr 패키지를 다시 설치하세요.", "このシステム用の TUI が含まれていません。最新の Quosparr を再インストールしてください。", "В приложении нет подходящего TUI. Переустановите последний пакет Quosparr.", "L’application ne contient pas de TUI adapté au système. Réinstallez le dernier paquet Quosparr."},
		xTUIInstallConflict:    {"Another file already occupies the quosparr command path. Move it before installing TUI; it was not overwritten.", "quosparr 命令位置已有其他文件，请先移走再安装；原文件未被覆盖。", "quosparr 命令位置已有其他檔案，請先移走再安裝；原檔案未被覆寫。", "quosparr 명령 경로에 다른 파일이 있습니다. 먼저 이동한 후 설치하세요. 기존 파일은 덮어쓰지 않았습니다.", "quosparr コマンドの場所に別のファイルがあります。移動してから再試行してください。既存ファイルは上書きしていません。", "Путь команды quosparr занят другим файлом. Переместите его перед установкой. Файл не перезаписан.", "Un autre fichier occupe le chemin de la commande quosparr. Déplacez-le avant l’installation ; il n’a pas été écrasé."},
	} {
		extraTexts[key] = texts
	}
}
