# 2026-08-10 — 移除 planner pm2 任務

`ecosystem.config.js` 原本註冊一個 `agy-gosdk-system` cron 任務（每日 0-9 時的第 40 分），
呼叫 `agy` 對本 repo 跑 `/system-planner`。本次移除整個檔案。

## 它從註冊那天起就不可能成功

任務的 `cwd` 與 `--add-dir` 都指向 `/Users/shuk/projects/tmp/gosdk`。
這個路徑`不存在`——gosdk 在 `platform/gosdk`，`~/projects/tmp/` 整個目錄都沒有。

問題不在於「路徑後來改了」，而在於`沒有人會發現`：

- 這支任務從未 `pm2 apply` 註冊過，所以 pm2 process list 裡看不到它；
- 就算註冊了，cron 任務失敗只會留在 pm2 log 裡，沒有 notifier；
- `ecosystem.config.js` 的存在本身，讓稽核時看起來「這個 repo 有排程」。

`真正的教訓`是 pm2 config 檔是`宣告`不是`事實`。檢查一個 repo 有沒有背景任務，
要看 `pm2 list` 的實際註冊狀態，不是看 repo 裡有沒有 `ecosystem.config.js`。
統一介面表把 `ecosystem.config.js` 列為`選備`正是這個道理：沒有常駐程序就不該有這個檔。

## 為什麼是刪檔而不是留空殼

中間狀態是把它改成 `module.exports = { apps: [] }`。這比原本更糟——
空的 apps 陣列讀起來像「刻意宣告沒有任務」，但實際上只是刪剩的殘骸，
下一個人得去翻 git log 才知道這裡曾經有什麼。既然這個 repo 是純函式庫、
沒有任何常駐程序，正確狀態就是`沒有這個檔`。

## 相關

- 同一天同樣理由移除了 `superset/ecosystem.config.js`（`agy-superset-system`，路徑同樣指向不存在的 `~/projects/tmp/superset`）。
