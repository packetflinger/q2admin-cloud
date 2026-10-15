// Activity charts: players and kills over time (lines), client versions and
// weapons (pies). The data is the JSON the page embeds in #q2a-stats-data.
(() => {
	const el = document.getElementById("q2a-stats-data");
	if (!el || !window.Chart) return;
	const st = JSON.parse(el.textContent);
	const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

	// times of day for spans up to two days, dates beyond that
	const longSpan = st.Times.length > 1 && (st.Times[1] - st.Times[0]) * st.Times.length > 2 * 86400;
	const labels = st.Times.map((t) => {
		const d = new Date(t * 1000);
		const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
		return longSpan ? `${d.toLocaleDateString([], { month: "short", day: "numeric" })} ${time}` : time;
	});

	let charts = [];
	const draw = () => {
		charts.forEach((c) => c.destroy());
		charts = [];
		const text = css("--bs-secondary-color");
		const grid = css("--bs-border-color");

		const line = (id, data, name) => {
			const canvas = document.getElementById(id);
			if (!canvas) return;
			const color = css("--q2a-series-1");
			charts.push(new Chart(canvas, {
				type: "line",
				data: { labels, datasets: [{ label: name, data, borderColor: color, backgroundColor: color, borderWidth: 2, pointRadius: 0, pointHoverRadius: 5, tension: 0.25 }] },
				options: {
					maintainAspectRatio: false,
					interaction: { mode: "index", intersect: false }, // hover anywhere on the x axis
					plugins: { legend: { display: false } }, // one series, the panel title names it
					scales: {
						x: { ticks: { color: text, maxTicksLimit: 6, maxRotation: 0 }, grid: { display: false } },
						y: { beginAtZero: true, ticks: { color: text, precision: 0 }, grid: { color: grid } },
					},
				},
			}));
		};

		const pie = (id, slices) => {
			const canvas = document.getElementById(id);
			if (!canvas || !slices) return;
			const total = slices.reduce((n, s) => n + s.Value, 0);
			charts.push(new Chart(canvas, {
				type: "pie",
				data: {
					labels: slices.map((s) => s.Label),
					datasets: [{
						data: slices.map((s) => s.Value),
						backgroundColor: slices.map((_, i) => css(`--q2a-series-${i + 1}`)),
						borderColor: css("--q2a-panel-bg"), // a surface gap between wedges
						borderWidth: 2,
					}],
				},
				options: {
					maintainAspectRatio: false,
					plugins: {
						legend: { display: false }, // the list under the chart is the legend
						tooltip: { callbacks: { label: (c) => `${c.parsed} (${Math.round((c.parsed * 100) / total)}%)` } },
					},
				},
			}));
		};

		line("q2a-players-chart", st.Players.map((v) => (v === null ? null : Math.round(v * 10) / 10)), "Players");
		line("q2a-kills-chart", st.Kills, "Kills");
		pie("q2a-versions-chart", st.Versions);
		pie("q2a-weapons-chart", st.Weapons);
	};
	draw();
	document.addEventListener("q2a-theme", draw);
})();
