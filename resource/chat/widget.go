package chat

import (
	"strconv"

	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/element"
)

// WidgetCfg configures one rendered chat widget instance.
type WidgetCfg struct {
	Channel string // validated channel key the widget binds to
	Title   string // heading; empty hides the heading bar
	Compact bool   // discussion strip (short, lighter) vs full page module
}

// RenderWidget appends a self-contained chat widget to the caller's builder.
// Self-contained on purpose — markup, scoped styles, and behavior travel
// together, so any module (or another resource, like the prayer wall) embeds
// chat with a single call and no asset-pipeline coordination.
//
// The widget is server-rendered as an empty shell; the JS below hydrates it:
//
//	load ── GET  /chat/messages?channel=X ──► render history + "me" controls
//	live ── SSE  /chat/stream?channel=X ────► append / delete / pin updates
//	post ── POST /chat/messages ────────────► server moderates, broadcasts
//
// Identity is deliberately NOT baked into the HTML: pages may be cached, and
// the logged-in/moderator distinction comes from the messages endpoint at
// hydration time instead.
func RenderWidget(b *element.Builder, cfg WidgetCfg) {
	if !ValidChannel(cfg.Channel) {
		// A bad channel is a placement/config error; render a marker comment
		// rather than a broken widget (and never pass junk to the JS).
		b.T("<!-- chat widget: invalid channel -->")
		return
	}

	// Unique DOM id per instance so two widgets can coexist on one page
	// (e.g. a community chat module plus an article discussion).
	wid := "chw" + auth.RandomKey()

	heightPx := 420
	if cfg.Compact {
		heightPx = 260
	}

	b.DivClass("ch-chat-widget", "id", wid, "data-channel", cfg.Channel).R(
		b.Wrap(func() {
			if cfg.Title != "" {
				b.DivClass("ch-chat-heading").T(cfg.Title)
			}
		}),
		b.DivClass("ch-chat-messages", "style", "height:"+strconv.Itoa(heightPx)+"px").R(
			b.DivClass("ch-chat-empty").T("Loading…"),
		),
		b.DivClass("ch-chat-error", "style", "display:none").R(),
		b.DivClass("ch-chat-compose", "style", "display:none").R(
			b.TextAreaClass("ch-chat-input", "rows", "2", "maxlength", strconv.Itoa(MaxMessageLen),
				"placeholder", "Write a message…").R(),
			b.ButtonClass("ch-chat-send", "type", "button").T("Send"),
		),
		b.DivClass("ch-chat-login-hint", "style", "display:none").R(
			b.A("href", "/login").T("Log in"),
			b.T(" to join the conversation"),
		),
		b.Style().T(widgetCSS),
		b.Script("type", "text/javascript").T(widgetJS(wid)),
	)
}

// widgetCSS is scoped under .ch-chat-widget so it cannot leak into site
// themes. Emitted per instance — duplicate <style> blocks are inert, and the
// few hundred bytes are cheaper than an asset-pipeline hook for one widget.
const widgetCSS = `
.ch-chat-widget { border: 1px solid #ddd; border-radius: 6px; margin: 0.75em 0; background: #fff; }
.ch-chat-widget .ch-chat-heading { padding: 0.5em 0.75em; font-weight: bold; border-bottom: 1px solid #eee; background: #f7f7f7; border-radius: 6px 6px 0 0; }
.ch-chat-widget .ch-chat-messages { overflow-y: auto; padding: 0.5em 0.75em; }
.ch-chat-widget .ch-chat-empty { color: #888; font-style: italic; padding: 0.5em 0; }
.ch-chat-widget .ch-chat-msg { padding: 0.3em 0; border-bottom: 1px dotted #eee; }
.ch-chat-widget .ch-chat-msg-kept { background: #fdf7e3; }
.ch-chat-widget .ch-chat-msg-head { font-size: 0.85em; color: #666; }
.ch-chat-widget .ch-chat-msg-author { font-weight: bold; color: #333; }
.ch-chat-widget .ch-chat-msg-body { white-space: pre-wrap; word-wrap: break-word; }
.ch-chat-widget .ch-chat-msg-tools { float: right; }
.ch-chat-widget .ch-chat-msg-tools button { border: none; background: none; cursor: pointer; font-size: 0.9em; opacity: 0.55; }
.ch-chat-widget .ch-chat-msg-tools button:hover { opacity: 1; }
.ch-chat-widget .ch-chat-compose { display: flex; gap: 0.5em; padding: 0.5em 0.75em; border-top: 1px solid #eee; }
.ch-chat-widget .ch-chat-input { flex: 1; resize: vertical; }
.ch-chat-widget .ch-chat-send { align-self: flex-end; }
.ch-chat-widget .ch-chat-error { color: #a00; padding: 0.25em 0.75em; font-size: 0.9em; }
.ch-chat-widget .ch-chat-login-hint { padding: 0.5em 0.75em; color: #666; border-top: 1px solid #eee; }
`

// widgetJS returns the hydration script for one widget instance. Vanilla JS
// (no jQuery dependency — not every page theme loads it). All user content
// enters the DOM via textContent, never innerHTML, so message bodies cannot
// inject markup.
func widgetJS(wid string) string {
	return `
(function() {
	var root = document.getElementById('` + wid + `');
	if (!root) return;
	var channel = root.getAttribute('data-channel');
	var list = root.querySelector('.ch-chat-messages');
	var compose = root.querySelector('.ch-chat-compose');
	var input = root.querySelector('.ch-chat-input');
	var sendBtn = root.querySelector('.ch-chat-send');
	var errBox = root.querySelector('.ch-chat-error');
	var loginHint = root.querySelector('.ch-chat-login-hint');
	var canModerate = false;
	var lastId = 0;

	function showErr(msg) {
		errBox.textContent = msg;
		errBox.style.display = msg ? 'block' : 'none';
		if (msg) setTimeout(function() { errBox.style.display = 'none'; }, 6000);
	}

	function fmtTime(iso) {
		var d = new Date(iso);
		return isNaN(d) ? '' : d.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
	}

	// Moderation tools re-check server-side; these buttons are convenience UI.
	function toolBtns(msg) {
		var tools = document.createElement('span');
		tools.className = 'ch-chat-msg-tools';
		var pin = document.createElement('button');
		pin.title = msg.keep ? 'Unkeep (allow expiry)' : 'Keep (save past 24h)';
		pin.textContent = msg.keep ? '★' : '☆'; /* filled/hollow star */
		pin.onclick = function() { modPost('/chat/keep/' + msg.id, 'keep=' + (!msg.keep)); };
		var del = document.createElement('button');
		del.title = 'Delete message';
		del.textContent = '✕';
		del.onclick = function() { modPost('/chat/delete/' + msg.id, ''); };
		tools.appendChild(pin);
		tools.appendChild(del);
		return tools;
	}

	function modPost(url, body) {
		fetch(url, {method: 'POST', credentials: 'same-origin',
			headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: body})
			.then(function(r) { return r.json().then(function(j) { if (!r.ok) showErr(j.error || 'Action failed'); }); })
			.catch(function() { showErr('Action failed'); });
	}

	function render(msg) {
		var row = document.createElement('div');
		row.className = 'ch-chat-msg' + (msg.keep ? ' ch-chat-msg-kept' : '');
		row.setAttribute('data-mid', msg.id);
		var head = document.createElement('div');
		head.className = 'ch-chat-msg-head';
		if (canModerate) head.appendChild(toolBtns(msg));
		var author = document.createElement('span');
		author.className = 'ch-chat-msg-author';
		author.textContent = msg.display_name || msg.username;
		head.appendChild(author);
		head.appendChild(document.createTextNode(' · ' + fmtTime(msg.created_at) + (msg.keep ? ' ★' : '')));
		var body = document.createElement('div');
		body.className = 'ch-chat-msg-body';
		body.textContent = msg.body;
		row.appendChild(head);
		row.appendChild(body);
		return row;
	}

	function append(msg) {
		if (msg.id <= lastId) return; // SSE + polling overlap guard
		lastId = msg.id;
		var empty = list.querySelector('.ch-chat-empty');
		if (empty) empty.remove();
		var nearBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60;
		list.appendChild(render(msg));
		if (nearBottom) list.scrollTop = list.scrollHeight;
	}

	function removeMsg(id) {
		var row = list.querySelector('[data-mid="' + id + '"]');
		if (row) row.remove();
		if (!list.querySelector('.ch-chat-msg')) setEmpty();
	}

	function setKept(id, keep) {
		var row = list.querySelector('[data-mid="' + id + '"]');
		if (!row) return;
		row.className = 'ch-chat-msg' + (keep ? ' ch-chat-msg-kept' : '');
		// Simplest correct refresh of star state: re-pull the window.
		load();
	}

	function setEmpty() {
		if (list.querySelector('.ch-chat-msg') || list.querySelector('.ch-chat-empty')) return;
		var d = document.createElement('div');
		d.className = 'ch-chat-empty';
		d.textContent = 'No messages yet — start the conversation!';
		list.appendChild(d);
	}

	function load() {
		fetch('/chat/messages?channel=' + encodeURIComponent(channel) + '&limit=50',
			{credentials: 'same-origin'})
			.then(function(r) { return r.json(); })
			.then(function(j) {
				if (j.error) { showErr(j.error); return; }
				canModerate = j.me && j.me.can_moderate;
				list.innerHTML = '';
				lastId = 0;
				(j.messages || []).forEach(append);
				setEmpty();
				list.scrollTop = list.scrollHeight;
				if (j.me && j.me.logged_in) {
					compose.style.display = 'flex';
					loginHint.style.display = 'none';
				} else {
					compose.style.display = 'none';
					loginHint.style.display = 'block';
				}
			})
			.catch(function() { showErr('Could not load messages'); });
	}

	function send() {
		var body = input.value.trim();
		if (!body) return;
		sendBtn.disabled = true;
		fetch('/chat/messages', {method: 'POST', credentials: 'same-origin',
			headers: {'Content-Type': 'application/x-www-form-urlencoded'},
			body: 'channel=' + encodeURIComponent(channel) + '&body=' + encodeURIComponent(body)})
			.then(function(r) { return r.json().then(function(j) { return {ok: r.ok, j: j}; }); })
			.then(function(res) {
				sendBtn.disabled = false;
				if (!res.ok) { showErr(res.j.error || 'Could not send'); return; }
				input.value = '';
				// Our own message arrives via SSE; append here too (deduped
				// by id in append) so the sender sees it even if SSE lags.
				if (res.j.message) append(res.j.message);
			})
			.catch(function() { sendBtn.disabled = false; showErr('Could not send'); });
	}

	sendBtn.onclick = send;
	input.addEventListener('keydown', function(e) {
		if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); }
	});

	// Live updates. The hub JSON-wraps as {type, data} on the default
	// "message" event; EventSource auto-reconnects on drops.
	if (window.EventSource) {
		var es = new EventSource('/chat/stream?channel=' + encodeURIComponent(channel));
		es.onmessage = function(evt) {
			var wrapped;
			try { wrapped = JSON.parse(evt.data); } catch (e) { return; }
			if (wrapped.type === 'chat_message') append(wrapped.data);
			else if (wrapped.type === 'chat_delete') removeMsg(wrapped.data.id);
			else if (wrapped.type === 'chat_keep') setKept(wrapped.data.id, wrapped.data.keep);
		};
	} else {
		// Ancient-browser fallback: poll for messages newer than what we have.
		setInterval(function() {
			fetch('/chat/messages?channel=' + encodeURIComponent(channel) + '&after_id=' + lastId,
				{credentials: 'same-origin'})
				.then(function(r) { return r.json(); })
				.then(function(j) { (j.messages || []).forEach(append); })
				.catch(function() {});
		}, 5000);
	}

	load();
})();
`
}
