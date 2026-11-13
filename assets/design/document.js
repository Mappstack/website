(function () {
    const iconsURL = new URL("icons.svg", document.currentScript.src).pathname;

    function readToken(name) {
        return parseFloat(getComputedStyle(document.documentElement).getPropertyValue(name));
    }

    function buildIcon(name, extra) {
        return '<svg class="icon' + (extra ? " " + extra : "") + '" aria-hidden="true"><use href="' + iconsURL + "#" + name + '"/></svg>';
    }

    function addHeadingAnchors() {
        document.querySelectorAll(".doc .prose h2[id], .doc .prose h3[id]").forEach(function (heading) {
            const anchor = document.createElement("a");
            anchor.className = "prose__anchor";
            anchor.href = "#" + heading.id;
            anchor.setAttribute("aria-label", "Copy link to " + heading.textContent.trim());
            anchor.dataset.copyLink = "";
            anchor.innerHTML = buildIcon("link") + buildIcon("check", "btn__swap");
            anchor.addEventListener("click", function () { history.replaceState(history.state, "", anchor.hash); });
            heading.prepend(anchor);
        });
    }

    function addCodeCopy() {
        document.querySelectorAll(".doc .prose pre").forEach(function (pre, index) {
            if (pre.parentElement.classList.contains("code-block")) return;
            const block = document.createElement("div");
            block.className = "code-block";
            const button = document.createElement("button");
            button.type = "button";
            button.className = "btn btn--icon btn--sm code-block__copy";
            button.setAttribute("aria-label", "Copy code");
            pre.id ||= "code-" + (index + 1);
            button.dataset.copy = pre.id;
            button.innerHTML = buildIcon("duplicate") + buildIcon("check", "btn__swap");
            pre.replaceWith(block);
            block.append(pre, button);
        });
    }

    function showReadingTime() {
        const target = document.querySelector('[data-role="reading-time"]');
        const body = document.querySelector(".doc__body");
        if (!target || !body) return;
        const words = body.textContent.trim().split(/\s+/).length;
        target.textContent = Math.max(1, Math.round(words / readToken("--reading-words-per-minute"))) + " min read";
    }

    function trackContents() {
        const contents = document.querySelector(".contents");
        if (!contents) return;
        const toggle = contents.querySelector(".contents__toggle");
        const current = toggle.querySelector("span");
        const list = contents.querySelector(".contents__list");
        const links = Array.from(list.querySelectorAll("a"));
        const headings = links.map(function (link) { return document.getElementById(decodeURIComponent(link.hash.slice(1))); });
        const wide = matchMedia("(min-width: 1024px)");
        let byKeyboard = false;
        list.id ||= "contents-list";
        list.popover = "auto";
        toggle.removeAttribute("aria-expanded");
        toggle.popoverTargetElement = list;
        function shownLinks() { return links.filter(function (link) { return link.checkVisibility(); }); }
        toggle.addEventListener("pointerdown", function () { byKeyboard = false; });
        toggle.addEventListener("keydown", function (event) {
            byKeyboard = true;
            if (event.key !== "ArrowDown" || list.matches(":popover-open")) return;
            event.preventDefault();
            list.showPopover();
        });
        list.addEventListener("toggle", function (event) {
            if (event.newState !== "open") return;
            const here = list.querySelector("[aria-current]") || shownLinks()[0];
            if (!here) return;
            here.scrollIntoView({ block: "nearest" });
            if (byKeyboard) here.focus();
        });
        list.addEventListener("keydown", function (event) {
            if (!list.matches(":popover-open")) return;
            const shown = shownLinks();
            const at = shown.indexOf(document.activeElement);
            const next = { ArrowDown: shown[Math.min(at + 1, shown.length - 1)], ArrowUp: shown[Math.max(at - 1, 0)], Home: shown[0], End: shown.at(-1) }[event.key];
            if (!next) return;
            event.preventDefault();
            next.focus();
        });
        links.forEach(function (link) {
            link.addEventListener("click", function () {
                if (list.matches(":popover-open")) list.hidePopover();
                const heading = document.getElementById(decodeURIComponent(link.hash.slice(1)));
                if (!heading) return;
                heading.tabIndex = -1;
                requestAnimationFrame(function () { heading.focus({ preventScroll: true }); });
            });
        });
        wide.addEventListener("change", function () { if (wide.matches && list.matches(":popover-open")) list.hidePopover(); });
        function keepInView(link) {
            const scroller = getComputedStyle(list).overflowY === "auto" ? list : contents;
            const box = scroller.getBoundingClientRect();
            const item = link.getBoundingClientRect();
            if (item.top < box.top || item.bottom > box.bottom) scroller.scrollTop += item.top - box.top - scroller.clientHeight / 2;
        }
        function markCurrent() {
            const offset = parseFloat(getComputedStyle(document.documentElement).scrollPaddingTop) + 8;
            const visible = headings.map(function (heading) { return Boolean(heading && heading.getClientRects().length); });
            let index = -1;
            headings.forEach(function (heading, position) { if (visible[position] && heading.getBoundingClientRect().top <= offset) index = position; });
            if (scrollY > 0 && innerHeight + scrollY >= document.documentElement.scrollHeight - 2) index = visible.lastIndexOf(true);
            links.forEach(function (link, position) {
                if (position === index) link.setAttribute("aria-current", "true");
                else link.removeAttribute("aria-current");
            });
            current.textContent = index >= 0 ? links[index].textContent : "";
            if (index >= 0) keepInView(links[index]);
        }
        let frame = 0;
        function queueMark() { frame ||= requestAnimationFrame(function () { frame = 0; markCurrent(); }); }
        addEventListener("scroll", queueMark, { passive: true });
        addEventListener("resize", queueMark);
        addEventListener("hashchange", queueMark);
        document.addEventListener("doc:filter", queueMark);
        markCurrent();
    }

    function escapePattern(term) {
        return term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    }

    function clearMarks(root) {
        root.querySelectorAll("mark").forEach(function (mark) { mark.replaceWith(mark.textContent); });
        root.normalize();
    }

    function markTerms(root, pattern) {
        const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
        const nodes = [];
        while (walker.nextNode()) nodes.push(walker.currentNode);
        nodes.forEach(function (node) {
            const parts = node.textContent.split(pattern);
            if (parts.length < 2) return;
            const fragment = document.createDocumentFragment();
            parts.forEach(function (part, position) {
                if (position % 2) {
                    const mark = document.createElement("mark");
                    mark.textContent = part;
                    fragment.append(mark);
                } else if (part) fragment.append(part);
            });
            node.replaceWith(fragment);
        });
    }

    function filterItems() {
        const field = document.querySelector('[data-role="doc-filter"]');
        if (!field) return;
        const items = Array.from(document.querySelectorAll(".doc__item"));
        const groups = Array.from(document.querySelectorAll(".doc__group"));
        const noResults = document.querySelector(".doc__noresults");
        const noResultsText = document.querySelector('[data-role="doc-noresults"]');
        function contentsEntry(attribute, value) {
            return document.querySelector(".contents__list [" + attribute + '="' + value + '"]');
        }
        field.addEventListener("input", function () {
            const query = field.value.trim();
            const terms = query.split(/\s+/).filter(Boolean).map(escapePattern);
            const tests = terms.map(function (term) { return new RegExp("\\b" + term, "i"); });
            const pattern = new RegExp("\\b(" + terms.join("|") + ")", "gi");
            let shown = 0;
            items.forEach(function (item) {
                clearMarks(item);
                const match = tests.every(function (test) { return test.test(item.textContent); });
                item.hidden = !match;
                const entry = contentsEntry("data-item", item.dataset.item);
                if (entry) entry.hidden = !match;
                if (match && terms.length) markTerms(item, pattern);
                if (match) shown += 1;
            });
            groups.forEach(function (group) {
                const empty = !group.querySelector(".doc__item:not([hidden])");
                group.hidden = empty;
                const entry = contentsEntry("data-group", group.dataset.group);
                if (entry) entry.hidden = empty;
            });
            if (noResults) noResults.hidden = shown > 0;
            if (noResultsText) noResultsText.textContent = "Nothing mentions “" + query + "”. Try fewer words.";
            design.announce(shown ? (terms.length ? shown + " of " : "") + items.length + " questions" : "No results for “" + query + "”");
            document.dispatchEvent(new Event("doc:filter"));
        });
    }

    addHeadingAnchors();
    addCodeCopy();
    showReadingTime();
    trackContents();
    filterItems();
})();
