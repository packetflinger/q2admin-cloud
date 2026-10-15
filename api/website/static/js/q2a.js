// Theme toggle: dark by default, the choice is remembered per browser.
(() => {
	const root = document.documentElement;
	const button = document.getElementById("q2a-theme-toggle");
	if (!button) return;
	const showIcon = () => {
		// show what clicking will switch to
		button.querySelector("i").className = root.dataset.bsTheme === "dark" ? "bi bi-sun" : "bi bi-moon-stars";
	};
	showIcon();
	button.addEventListener("click", () => {
		root.dataset.bsTheme = root.dataset.bsTheme === "dark" ? "light" : "dark";
		try { localStorage.setItem("q2a-theme", root.dataset.bsTheme); } catch (e) {}
		showIcon();
	});
})();
