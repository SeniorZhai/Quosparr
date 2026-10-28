package app

func init() {
	extraTexts[xRemoteReplace] = [7]string{"Replace key", "更换密钥", "更換金鑰", "키 교체", "キーを変更", "Заменить ключ", "Remplacer la clé"}

	extraTexts[xRemoteUnconfigured] = [7]string{
		"Add a key to show this quota.", "填写密钥后显示额度。", "填寫金鑰後顯示額度。", "키를 입력하면 사용량이 표시됩니다.", "キーを入力すると利用状況を表示します。", "Введите ключ, чтобы показать квоту.", "Ajoutez une clé pour afficher ce quota.",
	}
	extraTexts[xRemoteUnauthorized] = [7]string{
		"The key was rejected.", "密钥被拒绝。", "金鑰被拒絕。", "키가 거부되었습니다.", "キーが拒否されました。", "Ключ отклонён.", "La clé a été refusée.",
	}
	extraTexts[xRemoteFailed] = [7]string{
		"Could not read the quota.", "无法读取额度。", "無法讀取額度。", "사용량을 읽지 못했습니다.", "利用状況を読み取れません。", "Не удалось прочитать квоту.", "Impossible de lire le quota.",
	}
	extraTexts[xRemoteUnavailable] = [7]string{
		"This balance is unavailable.", "余额不可用。", "餘額不可用。", "잔액을 사용할 수 없습니다.", "残高は利用できません。", "Баланс недоступен.", "Ce solde est indisponible.",
	}
	extraTexts[xRemoteAPIKey] = [7]string{
		"API key", "API Key", "API Key", "API 키", "APIキー", "API-ключ", "Clé API",
	}
	extraTexts[xRemoteToken] = [7]string{
		"GitHub token", "GitHub token", "GitHub token", "GitHub 토큰", "GitHubトークン", "Токен GitHub", "Jeton GitHub",
	}
	extraTexts[xRemoteRegionIntl] = [7]string{
		"International (kimi.ai)", "国际区（kimi.ai）", "國際區（kimi.ai）", "해외 (kimi.ai)", "海外 (kimi.ai)", "Международный (kimi.ai)", "International (kimi.ai)",
	}
	extraTexts[xRemoteSave] = [7]string{
		"Save", "保存", "儲存", "저장", "保存", "Сохранить", "Enregistrer",
	}
	extraTexts[xRemoteClear] = [7]string{
		"Clear", "清除", "清除", "지우기", "消去", "Очистить", "Effacer",
	}
	extraTexts[xRemoteSaved] = [7]string{
		"Saved", "已保存", "已儲存", "저장됨", "保存しました", "Сохранено", "Enregistré",
	}
	extraTexts[xRemoteHint] = [7]string{
		"The key stays in Quosparr. It is not written to the provider's own login.", "密钥只保存在 Quosparr，不会写入该服务自己的登录文件。", "金鑰只保存在 Quosparr，不會寫入該服務自己的登入檔。", "키는 Quosparr에만 저장되며 해당 서비스의 로그인 파일에는 쓰지 않습니다.", "キーは Quosparr だけに保存され、各サービスのログインファイルには書き込みません。", "Ключ хранится только в Quosparr и не записывается в файл входа сервиса.", "La clé reste dans Quosparr. Elle n’est pas écrite dans la connexion du service.",
	}
	extraTexts[xRemoteConfigured] = [7]string{
		"A key is saved. Enter a new one to replace it.", "已保存密钥。输入新密钥可替换。", "已儲存金鑰。輸入新金鑰可替換。", "키가 저장되어 있습니다. 새 키를 입력하면 바뀝니다.", "キーは保存済みです。新しいキーを入力すると置き換わります。", "Ключ сохранён. Введите новый, чтобы заменить его.", "Une clé est enregistrée. Saisissez-en une nouvelle pour la remplacer.",
	}
	extraTexts[xRemoteGranted] = [7]string{
		"Granted", "赠送", "贈送", "지급", "付与", "Начислено", "Offert",
	}
	extraTexts[xRemoteTopped] = [7]string{
		"Topped up", "充值", "儲值", "충전", "チャージ", "Пополнено", "Rechargé",
	}
	extraTexts[xRemoteCodeShare] = [7]string{
		"Code share of the monthly quota", "月额度中的代码份额", "月額度中的程式份额", "월 사용량 중 코드 몫", "月間枠のうちコード分", "Доля кода в месячной квоте", "Part de code du quota mensuel",
	}
	extraTexts[xRemoteDeficit] = [7]string{
		"Deficit", "欠费", "欠費", "마이너스", "不足", "Дефицит", "Déficit",
	}
	extraTexts[xRemoteSettings] = [7]string{
		"Kimi, DeepSeek and Copilot", "Kimi、DeepSeek 和 Copilot", "Kimi、DeepSeek 和 Copilot", "Kimi, DeepSeek, Copilot", "Kimi、DeepSeek、Copilot", "Kimi, DeepSeek и Copilot", "Kimi, DeepSeek et Copilot",
	}
}

const (
	xRemoteReplace      textKey = "remoteReplace"
	xRemoteUnconfigured textKey = "remoteUnconfigured"
	xRemoteUnauthorized textKey = "remoteUnauthorized"
	xRemoteFailed       textKey = "remoteFailed"
	xRemoteUnavailable  textKey = "remoteUnavailable"
	xRemoteAPIKey       textKey = "remoteAPIKey"
	xRemoteToken        textKey = "remoteToken"
	xRemoteRegionIntl   textKey = "remoteRegionIntl"
	xRemoteSave         textKey = "remoteSave"
	xRemoteClear        textKey = "remoteClear"
	xRemoteSaved        textKey = "remoteSaved"
	xRemoteHint         textKey = "remoteHint"
	xRemoteConfigured   textKey = "remoteConfigured"
	xRemoteGranted      textKey = "remoteGranted"
	xRemoteTopped       textKey = "remoteTopped"
	xRemoteCodeShare    textKey = "remoteCodeShare"
	xRemoteDeficit      textKey = "remoteDeficit"
	xRemoteSettings     textKey = "remoteSettings"
)
