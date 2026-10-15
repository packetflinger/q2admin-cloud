// Live server console: output streams in over server-sent events, commands
// go out as form POSTs.
(() => {
	const el = document.getElementById("console");
	const form = document.getElementById("console-form");
	const input = document.getElementById("console-input");
	const status = document.getElementById("console-status");
	if (!el || !form) return;

	const term = new Terminal({
		convertEol: true,
		disableStdin: true,
		fontSize: window.innerWidth < 576 ? 12 : 14,
		scrollback: 5000,
		theme: { background: "#0b0d12" },
	});
	const fit = new FitAddon.FitAddon();
	term.loadAddon(fit);
	term.open(el);
	fit.fit();
	window.addEventListener("resize", () => fit.fit());

	const time = () => new Date().toLocaleTimeString([], { hour12: false });
	const setStatus = (text, cls) => {
		status.textContent = text;
		status.className = "badge " + cls;
	};

	// EventSource reconnects on its own after a drop
	const events = new EventSource(el.dataset.stream);
	events.onopen = () => setStatus("live", "text-bg-success");
	events.onerror = () => setStatus("reconnecting...", "text-bg-warning");
	events.onmessage = (e) => term.writeln(`\x1b[90m${time()}\x1b[0m ${JSON.parse(e.data).trimEnd()}`);

	const history = [];
	let pos = 0;
	input.addEventListener("keydown", (e) => {
		if (e.key === "ArrowUp" && pos > 0) {
			input.value = history[--pos];
		} else if (e.key === "ArrowDown" && pos < history.length) {
			input.value = history[++pos] ?? "";
		} else {
			return;
		}
		e.preventDefault();
	});

	form.addEventListener("submit", async (e) => {
		e.preventDefault();
		const cmd = input.value.trim();
		if (!cmd) return;
		history.push(cmd);
		pos = history.length;
		input.value = "";
		try {
			const res = await fetch(form.dataset.command, { method: "POST", body: new URLSearchParams({ cmd }) });
			if (!res.ok) term.writeln(`\x1b[31m${(await res.text()).trim()}\x1b[0m`);
		} catch (err) {
			term.writeln(`\x1b[31mcommand not sent: ${err.message}\x1b[0m`);
		}
	});
})();
