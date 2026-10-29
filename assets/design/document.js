(function () {
    const iconsURL = new URL("icons.svg", document.currentScript.src).pathname;
    const wordsPerMinute = 230;

    function buildIcon(name) {
        return '<svg class="icon" aria-hidden="true"><use href="' + iconsURL + "#" + name + '"/></svg>';
    }

    function showToast(message) {
        const toast = document.createElement("div");
        toast.className = "toast toast--floating";
        toast.setAttribute("role", "status");
        toast.textContent = message;
        document.body.append(toast);
        setTimeout(function () { toast.remove(); }, 1600);
    }

    function addHeadingAnchors() {
        document.querySelectorAll(".doc .prose h2[id], .doc .prose h3[id]").forEach(function (heading) {
            const anchor = document.createElement("a");
            anchor.className = "prose__anchor";
            anchor.href = "#" + heading.id;
            anchor.setAttribute("aria-label", "Copy link to " + heading.textContent.trim());
            anchor.innerHTML = buildIcon("link");
            anchor.addEventListener("click", function (event) {
                event.preventDefault();
                history.replaceState(null, "", anchor.hash);
                navigator.clipboard.writeText(location.href).then(function () { showToast("Link Copied"); });
            });
            heading.prepend(anchor);
        });
    }

    function addCodeCopy() {
        document.querySelectorAll(".doc .prose pre").forEach(function (pre) {
            if (pre.parentElement.classList.contains("code-block")) return;
            const block = document.createElement("div");
            block.className = "code-block";
            const button = document.createElement("button");
            button.type = "button";
            button.className = "btn btn--icon btn--sm code-block__copy";
            button.setAttribute("aria-label", "Copy code");
            button.innerHTML = buildIcon("duplicate");
            button.addEventListener("click", function () {
                navigator.clipboard.writeText(pre.textContent).then(function () {
                    button.innerHTML = buildIcon("check");
                    showToast("Code Copied");
                    setTimeout(function () { button.innerHTML = buildIcon("duplicate"); }, 1600);
                });
            });
            pre.replaceWith(block);
            block.append(pre, button);
        });
    }

    function showReadingTime() {
        const target = document.querySelector('[data-role="reading-time"]');
        const body = document.querySelector(".doc__body");
        if (!target || !body) return;
        const words = body.textContent.trim().split(/\s+/).length;
        target.textContent = Math.max(1, Math.round(words / wordsPerMinute)) + " min read";
    }

    function trackContents() {
        const contents = document.querySelector(".contents");
        if (!contents) return;
        const toggle = contents.querySelector(".contents__toggle");
        const current = toggle.querySelector("span");
        const list = contents.querySelector(".contents__list");
        const links = Array.from(list.querySelectorAll("a"));
        const headings = links.map(function (link) { return document.getElementById(decodeURIComponent(link.hash.slice(1))); });
        function setOpen(open) {
            contents.classList.toggle("is-open", open);
            toggle.setAttribute("aria-expanded", String(open));
        }
        toggle.addEventListener("click", function () { setOpen(!contents.classList.contains("is-open")); });
        links.forEach(function (link) { link.addEventListener("click", function () { setOpen(false); }); });
        document.addEventListener("click", function (event) { if (!contents.contains(event.target)) setOpen(false); });
        document.addEventListener("keydown", function (event) {
            if (event.key !== "Escape" || !contents.classList.contains("is-open")) return;
            setOpen(false);
            toggle.focus();
        });
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
            document.dispatchEvent(new Event("doc:filter"));
        });
    }

    addHeadingAnchors();
    addCodeCopy();
    showReadingTime();
    trackContents();
    filterItems();
})();
