package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/store"
)

func (u *settingsUI) claudeSSHGroup(c *ui.Context) {
	l := u.L()
	if !u.claudeSSHReady {
		u.claudeSSHHost = u.s.Prefs().ClaudeSSHHost
		u.claudeSSHReady = true
	}
	settingsGroup(c, tx(l, xClaudeSSHSource), tx(l, xClaudeSSHHelp), nil, func(row rowFunc) {
		row(tx(l, xClaudeSSHHost), "", func() {
			ui.TextInput(c.Key("claude-ssh-host"), &u.claudeSSHHost).Width(180).Label(tx(l, xClaudeSSHHost)).Placeholder("macmini").Disabled(u.s.ReadOnly())
		})
		row("", "", func() {
			host := strings.TrimSpace(u.claudeSSHHost)
			invalid := host != "" && !claude.ValidSSHHost(host)
			if PillButton(c, nil, tx(l, xClaudeSSHSave), u.s.ReadOnly() || invalid).Clicked() {
				if u.s.Update(func(p *store.Preferences) { p.ClaudeSSHHost = host }) && u.opts.Actions.Refresh != nil {
					u.opts.Actions.Refresh()
				}
			}
		})
	})
}

const (
	xClaudeSSHSource textKey = "claudeSSHSource"
	xClaudeSSHHelp   textKey = "claudeSSHHelp"
	xClaudeSSHHost   textKey = "claudeSSHHost"
	xClaudeSSHSave   textKey = "claudeSSHSave"
	xClaudeSSHFailed textKey = "claudeSSHFailed"
)

func init() {
	for key, texts := range map[textKey][7]string{
		xClaudeSSHSource: {"Claude: SSH quota · local statistics", "Claude：SSH 额度 · 本地统计", "Claude：SSH 額度 · 本機統計", "Claude: SSH 할당량 · 로컬 통계", "Claude: SSH 利用枠 · ローカル統計", "Claude: квота SSH · локальная статистика", "Claude : quota SSH · statistiques locales"},
		xClaudeSSHHost:   {"SSH host alias", "SSH 主机别名", "SSH 主機別名", "SSH 호스트 별칭", "SSH ホストエイリアス", "Псевдоним хоста SSH", "Alias d’hôte SSH"},
		xClaudeSSHSave:   {"Save and refresh", "保存并刷新", "儲存並重新整理", "저장 및 새로 고침", "保存して更新", "Сохранить и обновить", "Enregistrer et actualiser"},
		xClaudeSSHHelp:   {"Leave blank for local usage. SSH requires an existing trusted connection, Python 3 and Claude file credentials on the remote host. Tokens stay remote. SSH and HTTP use the remote connection settings, not this app’s proxy. Quota comes from SSH; token usage, cost and sessions come from local logs. Local accounts and Widget stay separate.", "留空使用本机来源。需要已信任的 SSH 连接、远端 Python 3 和 Claude 文件凭据。Token 留在远端；SSH 与远端 HTTP 使用各自连接配置，不使用本应用代理。额度来自 SSH；Token、成本与会话来自本机日志。本机账号与 Widget 保持独立。", "留空使用本機來源。需要已信任的 SSH 連線、遠端 Python 3 與 Claude 檔案憑據。Token 留在遠端；SSH 與遠端 HTTP 使用各自連線設定，不使用本應用程式代理。額度來自 SSH；Token、成本與會話來自本機日誌。本機帳號與 Widget 保持獨立。", "비워 두면 로컬 사용량을 표시합니다. 신뢰된 SSH 연결, 원격 Python 3 및 Claude 파일 자격 증명이 필요합니다. 토큰은 원격에 유지됩니다. SSH와 HTTP는 앱 프록시가 아닌 원격 연결 설정을 사용합니다. 할당량은 SSH에서, 토큰·비용·세션은 로컬 로그에서 가져옵니다. 로컬 계정과 Widget은 별도입니다.", "空欄でローカルに戻ります。信頼済み SSH 接続、リモートの Python 3 と Claude 認証ファイルが必要です。トークンはリモートに保持されます。SSH と HTTP はアプリのプロキシではなく接続先の設定を使用します。枠は SSH、トークン・費用・セッションはローカルログから取得します。ローカルアカウントと Widget は独立します.", "Пустое поле возвращает локальный источник. Нужны доверенное SSH-соединение, Python 3 и файл учётных данных Claude на сервере. Токены остаются на сервере. SSH и HTTP используют настройки соединения, а не прокси приложения. Квота поступает через SSH, токены, стоимость и сеансы — из локальных журналов. Локальные аккаунты и Widget независимы.", "Laissez vide pour la source locale. Une connexion SSH approuvée, Python 3 et les identifiants Claude dans un fichier distant sont requis. Les jetons restent distants. SSH et HTTP utilisent leurs paramètres de connexion, pas le proxy de l’application. Le quota vient de SSH ; les tokens, coûts et sessions viennent des journaux locaux. Comptes locaux et Widget restent distincts."},
		xClaudeSSHFailed: {"Remote usage unavailable. Check SSH, Python 3 and Claude login on the remote host, then refresh.", "远端用量不可用。请检查 SSH、远端 Python 3 和 Claude 登录后刷新。", "遠端用量無法取得。請檢查 SSH、遠端 Python 3 與 Claude 登入後重新整理。", "원격 사용량을 사용할 수 없습니다. SSH, 원격 Python 3 및 Claude 로그인을 확인하고 새로 고치세요.", "リモート使用量を取得できません。SSH、リモートの Python 3 と Claude ログインを確認して更新してください。", "Удалённые данные недоступны. Проверьте SSH, Python 3 и вход в Claude на сервере, затем обновите.", "Utilisation distante indisponible. Vérifiez SSH, Python 3 et la connexion Claude sur l’hôte distant, puis actualisez."},
	} {
		extraTexts[key] = texts
	}
}
