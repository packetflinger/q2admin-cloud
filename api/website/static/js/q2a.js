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

// Tabs marked data-q2a-remember open the tab named in the URL hash and keep
// the hash updated, so reloads and redirects land on the same tab.
document.querySelectorAll("[data-q2a-remember]").forEach((tabs) => {
	const fromHash = tabs.querySelector(`[data-bs-target="${CSS.escape(location.hash)}"]`);
	if (location.hash && fromHash) bootstrap.Tab.getOrCreateInstance(fromHash).show();
	tabs.addEventListener("shown.bs.tab", (e) => {
		history.replaceState(null, "", e.target.dataset.bsTarget);
	});
});

// Forms marked data-q2a-confirm ask before submitting.
document.querySelectorAll("form[data-q2a-confirm]").forEach((form) => {
	form.addEventListener("submit", (e) => {
		if (!confirm(form.dataset.q2aConfirm)) e.preventDefault();
	});
});
