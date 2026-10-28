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
	"terminalRequired": {"TUI mode requires an interactive terminal for input and output.", "TUI 模式需要可交互的终端输入和输出。", "TUI 模式需要可互動的終端輸入和輸出。", "TUI 모드에는 대화형 터미널 입력과 출력이 필요합니다.", "TUIモードには対話型端末の入出力が必要です。", "Режиму TUI нужен интерактивный терминал для ввода и вывода.", "Le mode TUI nécessite un terminal interactif en entrée et en sortie."},
	"terminalError":    {"Terminal error", "终端错误", "終端錯誤", "터미널 오류", "端末エラー", "Ошибка терминала", "Erreur de terminal"},
	"remaining":        {"Remaining", "剩余", "剩餘", "남음", "残り", "Осталось", "Restant"},
	"reset":            {"Reset in", "重置倒计时", "重置倒數", "초기화까지", "リセットまで", "До сброса", "Réinitialisation dans"},
	"resize":           {"Enlarge the terminal to see more", "扩大终端以显示更多", "放大終端以顯示更多", "더 보려면 터미널을 확대하세요", "端末を広げると続きを表示", "Увеличьте терминал, чтобы увидеть больше", "Agrandissez le terminal pour en voir plus"},
}
