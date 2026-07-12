// Shared modal shell used by todo/list/board edit forms. Centralizes
// open/close on htmx:afterSettle (same pattern board.js already uses for
// rebinding Sortable) instead of wiring hx-on::after-request on every edit
// button individually.
(function () {
	"use strict";

	const modal = document.getElementById("modal");
	const modalBody = document.getElementById("modal-body");
	if (!modal || !modalBody) return;

	// Tracks whether the user has explicitly dismissed the dialog since the
	// in-flight request that's populating it was issued. Without this, a
	// slow GET that resolves *after* the user already hit Esc would reopen
	// a dialog they just closed.
	let dismissed = false;

	document.body.addEventListener("htmx:beforeRequest", (e) => {
		if (e.target === modalBody) dismissed = false;
	});

	document.body.addEventListener("htmx:afterSettle", (e) => {
		if (e.target === modalBody && !dismissed && !modal.open) {
			modal.showModal();
		}
	});

	modal.addEventListener("click", (e) => {
		if (e.target === modal) modal.close(); // click landed on the backdrop
	});

	modal.addEventListener("close", () => {
		dismissed = true;
		modalBody.innerHTML = "";
	});
})();
