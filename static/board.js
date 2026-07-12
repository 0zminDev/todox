// Trello-like board: drag-and-drop (SortableJS) and a click/dropdown
// fallback both funnel through the same "move" persistence calls below.
// Both paths move the DOM client-side first, then fire a fire-and-forget
// POST so the server never needs to render HTML for a move — see
// internal/server/handlers_todos.go's handleTodoMove /
// internal/server/handlers_lists.go's handleListMove.
(function () {
	"use strict";

	const cardSortables = new Map(); // listID -> Sortable instance
	let boardSortable = null;

	function post(url, params) {
		return fetch(url, {
			method: "POST",
			headers: { "Content-Type": "application/x-www-form-urlencoded" },
			body: new URLSearchParams(params),
		});
	}

	function persistTodoMove(todoID, listID, afterID) {
		post("/todos/" + todoID + "/move", { list_id: listID, after_id: afterID || "" }).catch(
			(err) => console.error("failed to persist card move", err)
		);
	}

	function persistListMove(listID, afterID) {
		post("/lists/" + listID + "/move", { after_id: afterID || "" }).catch((err) =>
			console.error("failed to persist list move", err)
		);
	}

	function onCardDragEnd(evt) {
		const todoID = evt.item.dataset.todoId;
		const listID = evt.to.dataset.listId;
		const afterID = evt.item.previousElementSibling
			? evt.item.previousElementSibling.dataset.todoId
			: "";
		persistTodoMove(todoID, listID, afterID);
		refreshEmptyStates();
	}

	function onListDragEnd(evt) {
		const listID = evt.item.dataset.listId;
		const prev = evt.item.previousElementSibling;
		const afterID = prev && prev.dataset.listId ? prev.dataset.listId : "";
		persistListMove(listID, afterID);
	}

	// The server only renders a list's "no tasks yet" placeholder when it
	// renders the list from scratch. Dragging the last card out of a list
	// (or the first card into an empty one) changes emptiness client-side
	// without a server round-trip, so keep each list's placeholder in sync
	// here instead.
	function refreshEmptyStates() {
		document.querySelectorAll(".list-items").forEach((ul) => {
			const hasCards = ul.querySelector(".todo-item");
			const placeholder = ul.querySelector(".empty");
			if (!hasCards && !placeholder) {
				const li = document.createElement("li");
				li.className = "empty";
				li.id = "empty-state-" + ul.dataset.listId;
				li.textContent = "No tasks yet. Add one above.";
				ul.appendChild(li);
			} else if (hasCards && placeholder) {
				placeholder.remove();
			}
		});
	}

	function initCardSortable(ul) {
		const listID = ul.dataset.listId;
		if (cardSortables.has(listID)) return;
		cardSortables.set(
			listID,
			new Sortable(ul, {
				group: "cards",
				handle: ".card-drag-handle",
				animation: 150,
				onEnd: onCardDragEnd,
			})
		);
	}

	function pruneCardSortables() {
		for (const [listID, sortable] of cardSortables) {
			if (!document.body.contains(sortable.el)) {
				sortable.destroy();
				cardSortables.delete(listID);
			}
		}
	}

	function initBoardSortable() {
		const board = document.getElementById("lists-board");
		if (!board || boardSortable) return;
		boardSortable = new Sortable(board, {
			handle: ".list-drag-handle",
			animation: 150,
			onEnd: onListDragEnd,
		});
	}

	function populateMoveSelects() {
		const lists = Array.from(document.querySelectorAll(".list-column")).map((col) => ({
			id: col.dataset.listId,
			name: col.querySelector(".list-name").textContent,
		}));

		document.querySelectorAll(".move-select").forEach((select) => {
			const ul = select.closest(".list-items");
			const currentListID = ul ? ul.dataset.listId : null;

			select.innerHTML = "";
			lists.forEach((list) => {
				const option = document.createElement("option");
				option.value = list.id;
				option.textContent = list.name;
				if (list.id === currentListID) option.selected = true;
				select.appendChild(option);
			});
		});
	}

	function onMoveSelectChange(evt) {
		const select = evt.target;
		if (!select.matches(".move-select")) return;

		const todoID = select.dataset.todoId;
		const card = document.querySelector('[data-todo-id="' + todoID + '"].todo-item');
		const targetUl = document.getElementById("list-" + select.value + "-items");
		if (!card || !targetUl) return;

		const emptyPlaceholder = targetUl.querySelector(".empty");
		if (emptyPlaceholder) emptyPlaceholder.remove();
		targetUl.appendChild(card);

		persistTodoMove(todoID, select.value, "");
		refreshEmptyStates();
	}

	function refreshBoard() {
		document.querySelectorAll(".list-items").forEach(initCardSortable);
		pruneCardSortables();
		initBoardSortable();
		populateMoveSelects();
	}

	document.addEventListener("DOMContentLoaded", refreshBoard);
	document.body.addEventListener("htmx:afterSettle", refreshBoard);
	document.addEventListener("change", onMoveSelectChange);
})();
