(function () {
    const root = document.documentElement;
    const iconsURL = new URL("icons.svg", document.currentScript.src).pathname;

    const MillisecondsPerMinute = 60000;
    const FilterDelay = 160;
    const SearchDelay = 280;
    const SuggestDelay = 200;
    const StaggerLimit = 12;
    const MaskLength = 16;
    const DismissShare = 0.25;
    const DismissSpeed = 0.5;
    const Focusable = "a[href], button:not(:disabled), input:not(:disabled, [type=hidden]), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex='-1'])";
    const Fields = "input:not([type=hidden], [type=checkbox], [type=radio], [type=button], [type=submit], [type=reset], [type=file]), select, textarea";
    const TypingTypes = new Set(["text", "search", "email", "url", "tel", "password", "number", "date", "datetime-local", "month", "time", "week"]);

    const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");
    const finePointer = matchMedia("(pointer: fine)");
    const phone = matchMedia("(max-width: 720px)");
    const apple = /Mac|iPhone|iPad/.test(navigator.userAgentData ? navigator.userAgentData.platform : navigator.platform);

    let byKeyboard = false;

    function readToken(name) {
        return parseFloat(getComputedStyle(root).getPropertyValue(name));
    }

    function readValue(name) {
        return getComputedStyle(root).getPropertyValue(name).trim();
    }

    function buildIcon(name, extra) {
        return '<svg class="icon' + (extra ? " " + extra : "") + '" aria-hidden="true"><use href="' + iconsURL + "#" + name + '"/></svg>';
    }

    function isShown(element) {
        return element.checkVisibility();
    }

    function isTyping(element) {
        return Boolean(element && element.matches) && (element.isContentEditable || element.matches("textarea, select") || element.matches("input") && TypingTypes.has(element.type));
    }

    function settle(element, className) {
        element.addEventListener("animationend", function done(event) {
            if (event.target !== element) return;
            element.removeEventListener("animationend", done);
            element.classList.remove(className);
            element.style.removeProperty("--row-index");
        });
    }

    function statusIn(container) {
        let region = container.querySelector(":scope > [data-announcer]");
        if (region) return region;
        region = document.createElement("p");
        region.className = "visually-hidden";
        region.setAttribute("role", "status");
        region.dataset.announcer = "";
        container.append(region);
        return region;
    }

    function topModal() {
        const modals = Array.from(document.querySelectorAll("dialog:modal"));
        return modals.find(function (modal) { return modal.contains(document.activeElement); }) || modals.at(-1);
    }

    function announce(text) {
        const region = statusIn(topModal() || document.body);
        region.textContent = "";
        requestAnimationFrame(function () { region.textContent = text; });
    }

    function focusFirst(container) {
        const preferred = Array.from(container.querySelectorAll("[autofocus]")).find(function (element) { return !element.matches("button, [role=button]") && isShown(element); });
        const field = !container.matches(".dialog--alert") && (byKeyboard || finePointer.matches) && (preferred || Array.from(container.querySelectorAll(Fields)).find(isShown));
        (field || container).focus();
        if (field && field.matches("input") && TypingTypes.has(field.type) && field.value) field.select();
    }

    function firstShown(container, selector, skip) {
        if (container.matches(selector) && !container.matches(skip)) return container;
        return Array.from(container.querySelectorAll(selector)).find(function (element) { return isShown(element) && !element.matches(skip); });
    }

    function focusNear(element, fallback) {
        const neighbor = [element.nextElementSibling, element.previousElementSibling].find(function (sibling) { return sibling && isShown(sibling) && !sibling.matches(".is-leaving"); });
        const target = neighbor && (firstShown(neighbor, Fields, "[readonly], :disabled") || firstShown(neighbor, Focusable, ".btn--destructive, .menu-item--destructive"));
        if (target) target.focus();
        else if (fallback) fallback.focus();
    }

    function stagger(rows) {
        Array.from(rows).slice(0, StaggerLimit).forEach(function (row, index) {
            row.style.setProperty("--row-index", index);
            row.classList.add("is-new");
            settle(row, "is-new");
        });
    }

    function anchor(popover, invoker) {
        const name = "--" + CSS.escape(popover.id || "popover");
        invoker.style.anchorName = name;
        popover.style.positionAnchor = name;
    }

    const openers = new WeakMap();

    document.addEventListener("keydown", function () { byKeyboard = true; }, true);
    document.addEventListener("pointerdown", function () { byKeyboard = false; }, true);

    document.addEventListener("beforetoggle", function (event) {
        const dialog = event.target;
        if (!dialog.matches("dialog.dialog") || event.newState !== "open") return;
        if (!openers.has(dialog)) openers.set(dialog, document.activeElement);
        if (!dialog.hasAttribute("tabindex")) dialog.tabIndex = -1;
        if (!dialog.hasAttribute("closedby") && !dialog.matches(".dialog--alert")) dialog.setAttribute("closedby", "any");
    }, true);

    document.addEventListener("toggle", function (event) {
        const dialog = event.target;
        if (!dialog.matches("dialog.dialog")) return;
        if (event.newState === "open") {
            statusIn(dialog);
            focusFirst(dialog);
            return;
        }
        const opener = openers.get(dialog);
        openers.delete(dialog);
        const focused = document.activeElement;
        if (opener && opener.isConnected && isShown(opener) && (!focused || focused === document.body || dialog.contains(focused))) opener.focus();
    }, true);

    if (!("closedBy" in HTMLDialogElement.prototype)) {
        document.addEventListener("click", function (event) {
            const dialog = event.target;
            if (!dialog.matches || !dialog.matches("dialog.dialog[closedby=any][open]")) return;
            const box = dialog.getBoundingClientRect();
            if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) dialog.close();
        });
    }

    let sheet = null;
    let sheetLoading = false;
    const prefetched = new Map();

    function fetchFragment(url) {
        return fetch(url, { headers: { "X-Fragment": "1" }, redirect: "manual" }).then(function (response) {
            if (!response.ok || !(response.headers.get("content-type") || "").includes("text/html")) throw response;
            return response.text();
        });
    }

    function prefetch(url) {
        if (prefetched.has(url)) return;
        prefetched.clear();
        const pending = fetchFragment(url);
        pending.catch(function () {});
        prefetched.set(url, pending);
    }

    function requestFragment(url) {
        const pending = prefetched.get(url) || fetchFragment(url);
        prefetched.delete(url);
        return pending;
    }

    function parseContent(source) {
        if (typeof source === "string") {
            const template = document.createElement("template");
            template.innerHTML = source;
            return template.content;
        }
        if (source instanceof HTMLTemplateElement) return source.content.cloneNode(true);
        return source;
    }

    function readTitle(container) {
        const title = container.querySelector(".navbar__title, .dialog__title, .outcome__title");
        return title ? title.textContent.trim() : "";
    }

    function getSheet() {
        if (sheet) return sheet;
        sheet = document.createElement("dialog");
        sheet.id = "sheet";
        sheet.className = "dialog dialog--sheet";
        sheet.tabIndex = -1;
        sheet.setAttribute("closedby", "any");
        sheet.levels = [];
        document.body.append(sheet);
        sheet.addEventListener("click", function (event) {
            const cancel = event.target.closest("[data-cancel]");
            if (!cancel) return;
            event.preventDefault();
            if (sheet.levels.length) popLevel();
            else sheet.close();
        });
        sheet.addEventListener("keydown", function (event) {
            if (event.key !== "Escape" || event.defaultPrevented || !sheet.levels.length || sheet.querySelector(":popover-open")) return;
            event.preventDefault();
            popLevel();
        });
        sheet.addEventListener("close", function () {
            sheet.levels = [];
            setTimeout(function () { if (!sheet.open) sheet.replaceChildren(); }, readToken("--duration-state"));
        });
        return sheet;
    }

    function addBackLinks(title) {
        sheet.querySelectorAll(".navbar > .navbar__left[data-cancel], .navbar__left > [data-cancel]").forEach(function (control) {
            const link = document.createElement("a");
            link.href = "#";
            link.className = "navbar__left backlink";
            link.dataset.cancel = "";
            link.innerHTML = buildIcon("chevron-left", "icon--sm") + '<span class="backlink__label"></span>';
            link.lastChild.textContent = title || "Back";
            control.replaceWith(link);
        });
    }

    function moveLevel(from, direction) {
        if (!from || reducedMotion.matches) return;
        const to = sheet.offsetHeight;
        const duration = readToken("--duration-move");
        const easing = readValue("--ease-spring");
        if (from !== to) sheet.animate([{ height: from + "px" }, { height: to + "px" }], { duration: duration, easing: easing });
        if (!direction) return;
        const offset = direction === "back" ? "calc(var(--space-32) * -1)" : "var(--space-32)";
        Array.from(sheet.children).filter(function (child) { return !child.matches(".navbar"); }).forEach(function (child) {
            child.animate([{ opacity: 0, translate: offset + " 0" }, { opacity: 1, translate: "0 0" }], { duration: duration, easing: easing });
        });
    }

    function fillSheet(content, direction, fresh) {
        const from = sheet.open ? sheet.offsetHeight : 0;
        const region = sheet.querySelector(":scope > [data-announcer]");
        sheet.replaceChildren(content);
        if (region) sheet.append(region);
        sheet.setAttribute("aria-label", readTitle(sheet) || "Sheet");
        if (sheet.levels.length) addBackLinks(sheet.levels[sheet.levels.length - 1].title);
        if (fresh) {
            bindSearches(sheet);
            initTabs(sheet);
            sheet.dispatchEvent(new CustomEvent("design:content", { bubbles: true }));
        }
        moveLevel(from, direction);
    }

    function popLevel() {
        const level = sheet.levels.pop();
        fillSheet(level.nodes, "back", false);
        if (level.opener.isConnected && isShown(level.opener)) level.opener.focus();
        else focusFirst(sheet);
    }

    function showSheet(source, opener) {
        const dialog = getSheet();
        const content = parseContent(source);
        const pushing = dialog.open && opener && dialog.contains(opener);
        if (pushing) {
            const nodes = document.createDocumentFragment();
            nodes.append(...Array.from(dialog.childNodes).filter(function (node) { return !(node.matches && node.matches("[data-announcer]")); }));
            dialog.levels.push({ nodes: nodes, opener: opener, title: readTitle(nodes) });
        } else if (!dialog.open) {
            dialog.levels = [];
            if (opener) openers.set(dialog, opener);
        }
        fillSheet(content, pushing ? "forward" : "", true);
        if (!dialog.open) dialog.showModal();
        focusFirst(dialog);
        return new Promise(function (resolve) {
            dialog.addEventListener("close", function () { resolve(dialog.returnValue); }, { once: true });
        });
    }

    function showLoading(control) {
        if (!control) return function () {};
        control.setAttribute("aria-busy", "true");
        const chevron = control.querySelector(".cell__chevron") || control.closest(".cell") && control.closest(".cell").querySelector(".cell__chevron");
        const spinner = document.createElement("span");
        spinner.className = "spinner spinner--sm";
        const timer = chevron && setTimeout(function () { chevron.replaceWith(spinner); }, readToken("--delay-loading"));
        return function () {
            clearTimeout(timer);
            control.removeAttribute("aria-busy");
            if (spinner.isConnected) spinner.replaceWith(chevron);
        };
    }

    async function openSheet(source, opener) {
        if (typeof source !== "string" || source.trim().startsWith("<")) return showSheet(source, opener);
        if (sheetLoading) return;
        sheetLoading = true;
        const stopLoading = showLoading(opener);
        let html;
        try {
            html = await requestFragment(source);
        } catch (error) {
            location.href = source;
            return;
        } finally {
            sheetLoading = false;
            stopLoading();
        }
        return showSheet(html, opener);
    }

    document.addEventListener("pointerdown", function (event) {
        const link = event.target.closest("a[data-sheet]");
        if (link && event.button === 0) prefetch(link.getAttribute("href"));
    });

    document.addEventListener("click", function (event) {
        const link = event.target.closest("a[data-sheet]");
        if (!link || event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return;
        event.preventDefault();
        const menu = link.closest(".menu[popover]");
        openSheet(link.getAttribute("href"), menu && menuInvoker ? menuInvoker : link);
    });

    document.addEventListener("pointerdown", function (event) {
        const bar = event.target.closest(".dialog--sheet[open] > .navbar");
        if (!bar || event.button !== 0 || !phone.matches || event.target.closest("a, button, input, select, textarea")) return;
        const dialog = bar.parentElement;
        const start = event.clientY;
        const began = event.timeStamp;
        let distance = 0;
        bar.setPointerCapture(event.pointerId);
        dialog.style.transition = "none";
        function move(next) {
            distance = Math.max(0, next.clientY - start);
            dialog.style.translate = "0 " + distance + "px";
        }
        function end(last) {
            bar.removeEventListener("pointermove", move);
            bar.removeEventListener("pointerup", end);
            bar.removeEventListener("pointercancel", end);
            const speed = distance / Math.max(1, last.timeStamp - began);
            if (last.type !== "pointerup" || distance < dialog.offsetHeight * DismissShare && speed < DismissSpeed) {
                dialog.style.transition = "";
                dialog.style.translate = "";
                return;
            }
            const close = function () {
                dialog.close();
                dialog.style.transition = "";
                dialog.style.translate = "";
            };
            if (reducedMotion.matches) return close();
            dialog.style.translate = "0 100%";
            dialog.animate([{ translate: "0 " + distance + "px" }, { translate: "0 100%" }], { duration: readToken("--duration-move"), easing: readValue("--ease-standard") }).finished.then(close);
        }
        bar.addEventListener("pointermove", move);
        bar.addEventListener("pointerup", end);
        bar.addEventListener("pointercancel", end);
    });

    let menuInvoker = null;

    function menuItems(menu) {
        return Array.from(menu.querySelectorAll(".menu-item")).filter(function (item) {
            return item.matches("a[href], button:not(:disabled), [tabindex]") && !item.matches("[aria-disabled=true]") && isShown(item);
        });
    }

    function openMenuAt(menu, x, y) {
        menuInvoker = document.activeElement;
        menu.dataset.pointer = "";
        Object.assign(menu.style, { positionArea: "none", positionTryFallbacks: "none", margin: "0", left: x + "px", top: y + "px" });
        menu.showPopover();
        const box = menu.getBoundingClientRect();
        menu.style.left = Math.max(0, Math.min(x, innerWidth - box.width)) + "px";
        menu.style.top = Math.max(0, y + box.height > innerHeight ? y - box.height : y) + "px";
    }

    document.addEventListener("click", function (event) {
        const invoker = event.target.closest("[popovertarget]");
        if (invoker) menuInvoker = invoker;
    }, true);

    document.addEventListener("beforetoggle", function (event) {
        const menu = event.target;
        if (!menu.matches(".menu[popover]") || menu.matches(".menu--listbox")) return;
        if (event.newState === "open") {
            const invoker = event.source || menuInvoker;
            if (invoker && !("pointer" in menu.dataset)) anchor(menu, invoker);
        } else if ("pointer" in menu.dataset) {
            delete menu.dataset.pointer;
            ["positionArea", "positionTryFallbacks", "margin", "left", "top"].forEach(function (property) { menu.style[property] = ""; });
        }
    }, true);

    document.addEventListener("toggle", function (event) {
        const menu = event.target;
        if (!menu.matches(".menu[popover]") || menu.matches(".menu--listbox") || event.newState !== "open" || !byKeyboard) return;
        const items = menuItems(menu);
        const current = items.find(function (item) { return item.matches("[aria-checked=true], [aria-current], [aria-selected=true]"); });
        if (current || items[0]) (current || items[0]).focus();
    }, true);

    document.addEventListener("keydown", function (event) {
        const invoker = event.target.closest && event.target.closest("[popovertarget]");
        const target = invoker && invoker.popoverTargetElement;
        if (event.key !== "ArrowDown" || !target || !target.matches(".menu[popover]:not(:popover-open)")) return;
        event.preventDefault();
        menuInvoker = invoker;
        target.showPopover();
    });

    document.addEventListener("keydown", function (event) {
        const menu = event.target.closest && event.target.closest(".menu[popover]:popover-open:not(.menu--listbox)");
        if (!menu || event.metaKey || event.ctrlKey || event.altKey || isTyping(event.target) || !event.target.matches(".menu-item, .menu")) return;
        const items = menuItems(menu);
        const at = items.indexOf(document.activeElement);
        let next = { ArrowDown: items[(at + 1) % items.length], ArrowUp: items.at(at <= 0 ? -1 : at - 1), Home: items[0], End: items.at(-1) }[event.key];
        if (!next && event.key.length === 1 && event.key.trim()) {
            const letter = event.key.toLowerCase();
            next = items.slice(at + 1).concat(items.slice(0, at + 1)).find(function (item) { return item.textContent.trim().toLowerCase().startsWith(letter); });
        }
        if (!next) return;
        event.preventDefault();
        next.focus();
    });

    document.addEventListener("click", function (event) {
        const item = event.target.closest(".menu[popover] .menu-item");
        if (!item || item.closest(".menu--listbox") || item.matches("label") || item.querySelector("input")) return;
        const menu = item.closest(".menu[popover]");
        if (!menu.matches(":popover-open")) return;
        const returning = menu.contains(document.activeElement);
        menu.hidePopover();
        if (returning && menuInvoker && menuInvoker.isConnected && isShown(menuInvoker)) menuInvoker.focus();
    });

    document.addEventListener("focusout", function (event) {
        const menu = event.target.closest && event.target.closest(".menu[popover]:popover-open:not(.menu--listbox)");
        const next = event.relatedTarget;
        if (menu && next && !menu.contains(next) && next !== menuInvoker) menu.hidePopover();
    });

    function markMatches(label, query) {
        const fragment = document.createDocumentFragment();
        const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
        const lower = label.toLowerCase();
        const marks = Array.from(label, function () { return false; });
        terms.forEach(function (term) {
            let at = lower.indexOf(term);
            while (at >= 0) {
                if (at === 0 || /[\s\-(@.]/.test(lower[at - 1])) for (let index = at; index < at + term.length; index += 1) marks[index] = true;
                at = lower.indexOf(term, at + 1);
            }
        });
        wrapMarks(fragment, label, marks);
        return fragment;
    }

    function wrapMarks(fragment, text, marks) {
        let start = 0;
        for (let index = 1; index <= text.length; index += 1) {
            if (index < text.length && marks[index] === marks[start]) continue;
            const part = text.slice(start, index);
            if (marks[start]) {
                const mark = document.createElement("mark");
                mark.textContent = part;
                fragment.append(mark);
            } else fragment.append(part);
            start = index;
        }
    }

    function bindCombobox(input, source) {
        let list = document.getElementById(input.getAttribute("aria-controls"));
        if (!list) {
            list = document.createElement("div");
            list.id = (input.id || "combobox") + "-list";
            input.after(list);
            input.setAttribute("aria-controls", list.id);
        }
        list.classList.add("menu", "menu--listbox");
        list.setAttribute("role", "listbox");
        list.popover = "manual";
        input.setAttribute("role", "combobox");
        input.setAttribute("aria-autocomplete", "list");
        input.setAttribute("aria-expanded", "false");
        input.autocomplete = "off";
        const minimum = source.minimum || 0;
        let results = [];
        let active = -1;
        let sequence = 0;
        let timer = 0;
        let spoken = "";
        function options() { return Array.from(list.querySelectorAll("[role=option]")); }
        function close() {
            clearTimeout(timer);
            sequence += 1;
            if (list.matches(":popover-open")) list.hidePopover();
            input.setAttribute("aria-expanded", "false");
            input.removeAttribute("aria-activedescendant");
            active = -1;
            spoken = "";
        }
        function highlight(index) {
            const items = options();
            active = items[index] ? index : -1;
            items.forEach(function (item, position) { item.setAttribute("aria-selected", String(position === active)); });
            if (active < 0) {
                input.removeAttribute("aria-activedescendant");
                return;
            }
            input.setAttribute("aria-activedescendant", items[active].id);
            items[active].scrollIntoView({ block: "nearest" });
        }
        function usable() { return results.map(function (option, index) { return option.disabled ? -1 : index; }).filter(function (index) { return index >= 0; }); }
        function render(query) {
            list.replaceChildren(...results.map(function (option, index) {
                const item = document.createElement("div");
                item.className = "menu-item";
                item.id = list.id + "-" + index;
                item.setAttribute("role", "option");
                item.setAttribute("aria-selected", "false");
                if (option.disabled) item.setAttribute("aria-disabled", "true");
                item.append(markMatches(option.label, query));
                if (option.meta) {
                    const desc = document.createElement("span");
                    desc.className = "menu-item__desc";
                    desc.textContent = option.meta;
                    item.append(desc);
                }
                return item;
            }));
            if (!list.matches(":popover-open")) {
                anchor(list, input);
                list.showPopover();
            }
            input.setAttribute("aria-expanded", "true");
            const count = usable().length;
            const words = count === 1 ? "1 suggestion" : count ? count + " suggestions" : "No suggestions";
            if (words !== spoken) announce(words);
            spoken = words;
            highlight(query && count ? usable()[0] : -1);
        }
        async function search() {
            clearTimeout(timer);
            const query = input.value.trim();
            const turn = sequence += 1;
            if (query.length < minimum) return close();
            const found = await Promise.resolve(source.search(query)).catch(function () { return null; });
            if (turn !== sequence || document.activeElement !== input) return;
            if (!found) return close();
            results = found;
            render(query);
        }
        function choose(index) {
            const option = results[index];
            if (!option || option.disabled) return;
            close();
            source.choose(option);
        }
        function step(move) {
            const indexes = usable();
            if (!indexes.length) return;
            const at = indexes.indexOf(active);
            highlight(indexes[(at + move + indexes.length) % indexes.length]);
        }
        input.addEventListener("input", function () {
            clearTimeout(timer);
            timer = setTimeout(search, SuggestDelay);
        });
        input.addEventListener("click", function () { if (!minimum && !list.matches(":popover-open")) search(); });
        input.addEventListener("blur", close);
        input.addEventListener("keydown", function (event) {
            const open = list.matches(":popover-open");
            if (event.key === "ArrowDown") {
                event.preventDefault();
                if (open) step(1);
                else search();
            } else if (event.key === "ArrowUp" && open) {
                event.preventDefault();
                step(-1);
            } else if ((event.key === "Home" || event.key === "End") && open && usable().length) {
                event.preventDefault();
                highlight(event.key === "Home" ? usable()[0] : usable().at(-1));
            } else if (event.key === "Enter" && open && active >= 0) {
                event.preventDefault();
                choose(active);
            } else if (event.key === "Escape" && open) {
                event.preventDefault();
                close();
            }
        });
        list.addEventListener("mousedown", function (event) { event.preventDefault(); });
        list.addEventListener("pointermove", function (event) {
            const item = event.target.closest("[role=option]:not([aria-disabled=true])");
            if (item) highlight(options().indexOf(item));
        });
        list.addEventListener("click", function (event) {
            const item = event.target.closest("[role=option]");
            if (item) choose(options().indexOf(item));
        });
        return { close: close, search: search };
    }

    function glide(control, from, to) {
        if (!control.matches(".segmented") || reducedMotion.matches || from === to) return;
        const old = control.querySelector(".segmented__glide");
        if (old) old.remove();
        const thumb = document.createElement("span");
        thumb.className = "segmented__glide";
        control.classList.add("is-gliding");
        control.prepend(thumb);
        const animation = thumb.animate([{ left: from.offsetLeft + "px", width: from.offsetWidth + "px" }, { left: to.offsetLeft + "px", width: to.offsetWidth + "px" }], { duration: readToken("--duration-move"), easing: readValue("--ease-spring"), fill: "both" });
        animation.finished.catch(function () {}).then(function () {
            if (!thumb.isConnected) return;
            thumb.remove();
            control.classList.remove("is-gliding");
        });
    }

    function selectTab(tab, focus) {
        const list = tab.closest("[role=tablist]");
        const tabs = Array.from(list.querySelectorAll("[role=tab]"));
        const previous = tabs.find(function (item) { return item.getAttribute("aria-selected") === "true"; });
        tabs.forEach(function (item) {
            const on = item === tab;
            item.setAttribute("aria-selected", String(on));
            item.tabIndex = on ? 0 : -1;
            const panel = document.getElementById(item.getAttribute("aria-controls"));
            if (panel) panel.hidden = !on;
        });
        if (focus) tab.focus();
        if (previous && previous !== tab) glide(list, previous, tab);
    }

    function initTabs(scope) {
        scope.querySelectorAll("[role=tablist]").forEach(function (list) {
            const tabs = Array.from(list.querySelectorAll("[role=tab]"));
            const selected = tabs.find(function (tab) { return tab.getAttribute("aria-selected") === "true"; }) || tabs[0];
            tabs.forEach(function (tab) { tab.tabIndex = tab === selected ? 0 : -1; });
        });
    }

    document.addEventListener("click", function (event) {
        const tab = event.target.closest("[role=tab]");
        if (tab && !tab.matches("[aria-disabled=true]")) selectTab(tab, false);
    });

    document.addEventListener("keydown", function (event) {
        const tab = event.target.closest && event.target.closest("[role=tab]");
        if (!tab) return;
        const list = tab.closest("[role=tablist]");
        const tabs = Array.from(list.querySelectorAll("[role=tab]")).filter(isShown);
        const at = tabs.indexOf(tab);
        const vertical = list.getAttribute("aria-orientation") === "vertical";
        const next = { [vertical ? "ArrowDown" : "ArrowRight"]: tabs[(at + 1) % tabs.length], [vertical ? "ArrowUp" : "ArrowLeft"]: tabs.at(at - 1), Home: tabs[0], End: tabs.at(-1) }[event.key];
        if (!next) return;
        event.preventDefault();
        selectTab(next, true);
    });

    const choices = new WeakMap();

    function rememberChoice(event) {
        const control = event.target.closest && event.target.closest(".segmented");
        if (control && !choices.has(control)) choices.set(control, control.querySelector(".segmented__opt:has(:checked)"));
    }

    document.addEventListener("pointerdown", rememberChoice, true);
    document.addEventListener("keydown", rememberChoice, true);
    document.addEventListener("change", function (event) {
        const control = event.target.closest(".segmented");
        if (!control || !event.target.matches("input[type=radio]")) return;
        const from = choices.get(control);
        choices.delete(control);
        if (from) glide(control, from, event.target.closest(".segmented__opt"));
    });

    function readCopyText(target) {
        if ("value" in target.dataset) return target.dataset.value;
        if (target.matches("input, textarea, select")) return target.value;
        return target.textContent;
    }

    const copyTimers = new WeakMap();

    document.addEventListener("click", function (event) {
        const button = event.target.closest("[data-copy], [data-copy-link]");
        if (!button) return;
        event.preventDefault();
        const target = button.hasAttribute("data-copy-link") ? null : document.getElementById(button.dataset.copy);
        if (!button.hasAttribute("data-copy-link") && !target) return;
        navigator.clipboard.writeText(target ? readCopyText(target) : button.href).then(function () {
            clearTimeout(copyTimers.get(button));
            button.classList.add("is-copied");
            announce((button.getAttribute("aria-label") || "Copy").replace(/^Copy/, "Copied"));
            copyTimers.set(button, setTimeout(function () { button.classList.remove("is-copied"); }, readToken("--duration-notice")));
        }, function () {});
    });

    function setRevealed(button, on) {
        const target = document.getElementById(button.dataset.reveal);
        if (!target) return;
        target.textContent = on ? target.dataset.value : "•".repeat(MaskLength);
        button.setAttribute("aria-pressed", String(on));
    }

    document.addEventListener("click", function (event) {
        const button = event.target.closest("[data-reveal]");
        if (button) setRevealed(button, button.getAttribute("aria-pressed") !== "true");
    });

    document.addEventListener("visibilitychange", function () {
        if (document.hidden) document.querySelectorAll('[data-reveal][aria-pressed="true"]').forEach(function (button) { setRevealed(button, false); });
    });

    let toast = null;
    let toastTimer = 0;
    let toastAction = false;
    let toastReturn = null;

    function readNoticeDuration(message) {
        const words = message.trim().split(/\s+/).length;
        return Math.max(readToken("--duration-notice"), words / readToken("--reading-words-per-minute") * MillisecondsPerMinute);
    }

    function hideToast() {
        clearTimeout(toastTimer);
        if (!toast || toast.hidden) return;
        const returning = toast.contains(document.activeElement);
        toast.classList.add("is-leaving");
        setTimeout(function () { if (toast.classList.contains("is-leaving")) toast.hidden = true; }, readToken("--duration-state"));
        if (returning && toastReturn && toastReturn.isConnected) toastReturn.focus();
        toastReturn = null;
    }

    function armToast() {
        clearTimeout(toastTimer);
        if (!toastAction) toastTimer = setTimeout(hideToast, readNoticeDuration(toast.textContent));
    }

    function bindToast(element) {
        toast = element;
        toast.removeAttribute("role");
        toast.removeAttribute("aria-live");
        toast.addEventListener("pointerenter", function () { clearTimeout(toastTimer); });
        toast.addEventListener("focusin", function () { clearTimeout(toastTimer); });
        toast.addEventListener("pointerleave", function () { if (!toast.contains(document.activeElement)) armToast(); });
        toast.addEventListener("focusout", function (event) { if (!toast.contains(event.relatedTarget)) armToast(); });
        toast.addEventListener("keydown", function (event) {
            if (event.key !== "Escape") return;
            event.preventDefault();
            hideToast();
        });
    }

    function showToast(message, action) {
        if (!toast) {
            const element = document.createElement("div");
            element.className = "toast toast--floating";
            element.hidden = true;
            document.body.append(element);
            bindToast(element);
        }
        toast.classList.remove("is-leaving");
        toast.replaceChildren(message);
        toastAction = Boolean(action);
        if (action) {
            const button = document.createElement("button");
            button.type = "button";
            button.className = "toast__action";
            button.textContent = action.label;
            button.addEventListener("click", function () {
                hideToast();
                action.run();
            });
            toast.append(button);
        }
        toast.hidden = false;
        announce(action ? message + ". Press F6 for " + action.label + "." : message);
        armToast();
    }

    document.addEventListener("keydown", function (event) {
        if (event.key !== "F6" || !toast || toast.hidden || !toastAction) return;
        event.preventDefault();
        toastReturn = document.activeElement;
        toast.querySelector(".toast__action").focus();
    });

    function fold(input) {
        const to = Array.from(input.dataset.foldTo || "");
        const map = new Map(Array.from(input.dataset.foldFrom || "", function (character, index) { return [character, to[index]]; }));
        return function (text) { return Array.from(text || "", function (character) { return map.get(character) || character; }).join("").toLowerCase(); };
    }

    function clearMarks(element) {
        if (!element.querySelector("mark")) return;
        element.querySelectorAll("mark").forEach(function (mark) { mark.replaceWith(mark.textContent); });
        element.normalize();
    }

    function markRow(row, terms, folder) {
        row.querySelectorAll(".cell__title, .cell__meta").forEach(function (element) {
            const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
            const nodes = [];
            while (walker.nextNode()) nodes.push(walker.currentNode);
            nodes.forEach(function (node) {
                const text = node.textContent;
                const folded = folder(text);
                if (folded.length !== text.length) return;
                const marks = Array.from(text, function () { return false; });
                terms.forEach(function (term) {
                    let at = folded.indexOf(term);
                    while (at >= 0) {
                        if (term.length >= 3 || at === 0 || /\s/.test(folded[at - 1])) for (let index = at; index < at + term.length; index += 1) marks[index] = true;
                        at = folded.indexOf(term, at + 1);
                    }
                });
                if (!marks.includes(true)) return;
                const fragment = document.createDocumentFragment();
                wrapMarks(fragment, text, marks);
                node.replaceWith(fragment);
            });
        });
    }

    function matchesTerm(text, email, term) {
        if (term.length < 3) return (" " + text).includes(" " + term);
        if (/[@.]/.test(term)) return text.includes(term) || email.includes(term);
        return text.includes(term);
    }

    function findEmpty(list) {
        let node = list;
        while (node && !node.querySelector("[data-empty]")) node = node.parentElement;
        return node && node.querySelector("[data-empty]");
    }

    const bound = new WeakSet();

    function bindFilter(input, scope) {
        const list = scope.querySelector(input.dataset.list || ".list");
        if (!list) return;
        const folder = fold(input);
        const empty = findEmpty(list);
        const count = scope.querySelector("[data-count]");
        const noun = count ? count.dataset.noun : input.dataset.noun || "";
        const page = scope === document;
        const rows = Array.from(list.querySelectorAll(input.dataset.selector || ".list__row")).map(function (row) {
            const data = row.dataset;
            const keys = Object.keys(data).filter(function (key) { return key.startsWith("search"); });
            return { row: row, text: folder(keys.map(function (key) { return key === "searchEmail" ? data[key].split("@")[0] : data[key]; }).join(" ")), email: folder(data.searchEmail) };
        });
        let timer = 0;
        function apply(query, spoken) {
            const terms = [...new Set(folder(query).split(/\s+/).filter(Boolean))].slice(0, 6);
            let shown = 0;
            rows.forEach(function (entry) {
                const match = terms.every(function (term) { return matchesTerm(entry.text, entry.email, term); });
                entry.row.hidden = !match;
                clearMarks(entry.row);
                if (!match) return;
                shown += 1;
                if (terms.length) markRow(entry.row, terms, folder);
            });
            if (empty) {
                empty.hidden = shown > 0;
                empty.querySelectorAll("[data-query]").forEach(function (echo) { echo.textContent = query.trim(); });
            }
            const summary = ((terms.length ? shown + " of " : "") + rows.length + " " + noun).trim();
            if (count && rows.length) {
                count.hidden = count.hasAttribute("data-count-filtering") && !terms.length;
                count.textContent = summary;
            }
            if (page) {
                const params = new URLSearchParams(location.search);
                params.delete("search");
                if (query.trim()) params.set("q", query.trim());
                else params.delete("q");
                history.replaceState(history.state, "", location.pathname + (params.toString() ? "?" + params : ""));
            }
            list.dispatchEvent(new CustomEvent("filter:applied", { bubbles: true }));
            if (spoken) announce(shown ? summary : "No results for “" + query.trim() + "”");
        }
        function run(query) {
            if (reducedMotion.matches || !window.ViewTransition || !("types" in ViewTransition.prototype)) return apply(query, true);
            list.dataset.filtering = "";
            document.startViewTransition({ update: function () { apply(query, true); }, types: ["filter"] }).finished.finally(function () { delete list.dataset.filtering; });
        }
        if (page) {
            const params = new URLSearchParams(location.search);
            const query = params.get("q") || params.get("search");
            if (query) input.value = query;
        }
        input.addEventListener("keydown", function (event) {
            if (event.key === "Enter") event.preventDefault();
            if (event.key !== "Escape" || !input.value) return;
            event.preventDefault();
            input.value = "";
            clearTimeout(timer);
            run("");
        });
        input.addEventListener("input", function () {
            clearTimeout(timer);
            timer = setTimeout(function () { run(input.value); }, FilterDelay);
        });
        input.addEventListener("search", function () {
            clearTimeout(timer);
            run(input.value);
        });
        apply(input.value, false);
    }

    function bindLive(input, scope) {
        const embed = input.dataset.search === "embed";
        const form = embed ? null : input.form;
        const container = form || input.closest("[role=search]");
        const box = container && (embed ? input.closest("section") : scope).querySelector("[data-results]");
        const page = scope === document && !embed;
        if (!box) return;
        const picked = box.parentElement.querySelector("[data-picked]");
        let timer = 0;
        let controller = null;
        let sequence = 0;
        let current = location.pathname + location.search;
        function ready() { return input.value.trim().length !== 1; }
        function address() {
            const params = form ? new URLSearchParams(new FormData(form)) : new URLSearchParams([[input.name, input.value]]);
            Array.from(params.keys()).forEach(function (key) { if (!params.get(key).trim() || key === input.name && !ready()) params.delete(key); });
            const query = params.toString();
            return (container.getAttribute("action") || container.dataset.action || location.pathname) + (query ? "?" + query : "");
        }
        function describe() {
            const element = box.querySelector("[data-status], [data-total], .empty, .hint");
            return (element && element.textContent.trim().replace(/\s+/g, " ")) || "Results updated";
        }
        function keepPicked() {
            picked.querySelectorAll(".list__row:not([data-keep])").forEach(function (row) { if (!row.querySelector("input:is(.checkbox, .radio):checked")) row.remove(); });
            box.querySelectorAll("input:is(.checkbox, .radio):checked").forEach(function (check) { picked.append(check.closest(".list__row")); });
            picked.hidden = !picked.children.length;
        }
        function dropPickedDuplicates() {
            picked.querySelectorAll("input:is(.checkbox, .radio)").forEach(function (check) {
                const twin = box.querySelector('input:is(.checkbox, .radio)[value="' + CSS.escape(check.value) + '"]');
                if (twin) twin.closest(".list__row").remove();
            });
        }
        async function load(url, push) {
            if (controller) controller.abort();
            controller = new AbortController();
            const turn = sequence += 1;
            box.setAttribute("aria-busy", "true");
            try {
                const response = await fetch(url, { headers: { "X-Fragment": "1" }, signal: controller.signal, redirect: "manual" });
                if (turn !== sequence) return;
                if (!response.ok) {
                    location.href = url;
                    return;
                }
                const template = document.createElement("template");
                template.innerHTML = await response.text();
                if (turn !== sequence) return;
                if (picked) keepPicked();
                box.replaceChildren(...(template.content.querySelector("[data-results]") || template.content).childNodes);
                if (picked) dropPickedDuplicates();
                stagger(box.querySelectorAll(".list__row"));
                box.dispatchEvent(new CustomEvent("filter:applied", { bubbles: true, detail: { restored: push === null } }));
                const titled = box.querySelector("[data-title]");
                if (titled && page) document.title = titled.dataset.title;
                if (push !== null && page) history[push && url !== current ? "pushState" : "replaceState"]({ live: true }, "", url);
                current = url;
                announce(describe());
            } catch (error) {
                if (error.name !== "AbortError") location.href = url;
            } finally {
                if (turn === sequence) box.setAttribute("aria-busy", "false");
            }
        }
        input.addEventListener("input", function () {
            clearTimeout(timer);
            if (ready()) timer = setTimeout(function () { load(address(), false); }, SearchDelay);
        });
        input.addEventListener("keydown", function (event) {
            if (event.key === "Escape" && input.value) {
                event.preventDefault();
                input.value = "";
                clearTimeout(timer);
                load(address(), false);
            } else if (event.key === "Enter" && embed) {
                event.preventDefault();
                clearTimeout(timer);
                if (ready()) load(address(), false);
            }
        });
        if (form) {
            form.addEventListener("submit", function (event) {
                event.preventDefault();
                clearTimeout(timer);
                if (ready()) load(address(), true);
            });
            document.addEventListener("change", function (event) {
                if (event.target.form !== form || event.target === input) return;
                clearTimeout(timer);
                load(address(), true);
            });
        }
        if (!page) return;
        addEventListener("popstate", function () {
            if (location.pathname + location.search === current) return;
            const params = new URLSearchParams(location.search);
            Array.from(form.elements).forEach(function (field) { if (field.name) field.value = params.get(field.name) || ""; });
            load(location.pathname + location.search, null);
        });
        if (input.dataset.search === "live" && !input.value && matchMedia("(hover: hover)").matches) input.focus();
    }

    function bindSearches(scope) {
        scope.querySelectorAll("input[data-search]").forEach(function (input) {
            if (bound.has(input)) return;
            bound.add(input);
            if (input.dataset.search === "filter") bindFilter(input, scope);
            else bindLive(input, scope);
        });
    }

    document.addEventListener("click", function (event) {
        const button = event.target.closest("[data-clear-search]");
        if (!button) return;
        const field = document.getElementById(button.dataset.clearSearch) || document.querySelector("input[data-search]");
        if (!field) return;
        field.value = "";
        field.dispatchEvent(new Event("input", { bubbles: true }));
        field.dispatchEvent(new Event("search", { bubbles: true }));
        field.focus();
    });

    document.addEventListener("submit", function (event) {
        const button = event.submitter;
        const form = event.target;
        if (event.defaultPrevented || !button || (button.formMethod || form.method) === "dialog" || (button.formTarget || form.target || "_self") !== "_self") return;
        if (button.getAttribute("aria-busy") === "true" || button.getAttribute("aria-disabled") === "true") {
            event.preventDefault();
            return;
        }
        button.setAttribute("aria-busy", "true");
    });

    addEventListener("pageshow", function () {
        document.querySelectorAll('.btn[aria-busy="true"]').forEach(function (button) { button.removeAttribute("aria-busy"); });
    });

    function parseShortcuts(element) {
        return element.getAttribute("aria-keyshortcuts").split(/\s+/).filter(Boolean).map(function (combination) {
            const parts = combination.split("+");
            const key = parts.pop();
            return { key: key, meta: parts.includes("Meta"), control: parts.includes("Control"), shift: parts.includes("Shift"), alt: parts.includes("Alt") };
        });
    }

    function matchesShortcut(shortcut, event) {
        if (shortcut.meta !== event.metaKey || shortcut.control !== event.ctrlKey || shortcut.alt !== event.altKey) return false;
        if (/^[a-z]$/i.test(shortcut.key) && shortcut.shift !== event.shiftKey) return false;
        return event.key.toLowerCase() === shortcut.key.toLowerCase();
    }

    function shortcutControls() {
        const controls = Array.from(document.querySelectorAll("[aria-keyshortcuts]")).filter(function (control) { return isShown(control) && !control.matches(":disabled, [aria-disabled=true]"); });
        return controls.filter(function (control) { return control.closest("main"); }).concat(controls.filter(function (control) { return !control.closest("main"); }));
    }

    function describeShortcut(shortcut) {
        const symbols = apple ? { meta: "⌘", control: "⌃", alt: "⌥", shift: "⇧" } : { meta: "Win", control: "Ctrl", alt: "Alt", shift: "Shift" };
        const keys = ["control", "alt", "shift", "meta"].filter(function (name) { return shortcut[name]; }).map(function (name) { return symbols[name]; });
        return keys.concat(shortcut.key.length === 1 ? shortcut.key.toUpperCase() : shortcut.key);
    }

    function labelOf(control) {
        const own = Array.from(control.childNodes).filter(function (node) { return !(node.matches && node.matches(".kbd")); }).map(function (node) { return node.textContent; }).join("");
        const text = control.getAttribute("aria-label") || control.labels && control.labels[0] && control.labels[0].textContent || control.placeholder || own;
        return text.trim().replace(/\s+/g, " ");
    }

    function showShortcuts() {
        const claimed = new Set();
        const rows = shortcutControls().map(function (control) {
            const shortcuts = parseShortcuts(control).filter(function (shortcut) {
                const key = describeShortcut(shortcut).join("+");
                if ((apple ? shortcut.control && !shortcut.meta : shortcut.meta) || claimed.has(key)) return false;
                claimed.add(key);
                return true;
            });
            return [labelOf(control), shortcuts];
        }).filter(function (row) { return row[1].length; });
        rows.push(["Keyboard Shortcuts", [{ key: "?" }]], ["Close", [{ key: "Esc" }]]);
        const content = document.createElement("template");
        content.innerHTML = '<header class="navbar"><div class="navbar__left"></div><h2 class="navbar__center navbar__title">Keyboard Shortcuts</h2><div class="navbar__right"><button type="button" class="btn btn--plain" data-cancel>Done</button></div></header><div class="dialog__body"><ul class="list"></ul></div>';
        const list = content.content.querySelector(".list");
        rows.forEach(function (row) {
            const item = document.createElement("li");
            item.className = "list__row";
            item.innerHTML = '<div class="cell"><span class="cell__body"><span class="cell__title"></span></span><span class="cell__value"></span></div>';
            item.querySelector(".cell__title").textContent = row[0];
            const value = item.querySelector(".cell__value");
            row[1].forEach(function (shortcut, index) {
                if (index) value.append(" or ");
                describeShortcut(shortcut).forEach(function (key) {
                    const cap = document.createElement("kbd");
                    cap.className = "kbd";
                    cap.textContent = key;
                    value.append(cap);
                });
            });
            list.append(item);
        });
        openSheet(content.content, document.activeElement);
    }

    document.addEventListener("keydown", function (event) {
        if (event.defaultPrevented || event.altKey || event.isComposing || event.repeat) return;
        if (isTyping(event.target) || event.target.closest && event.target.closest(":popover-open") || document.querySelector("dialog:modal")) return;
        if (event.key === "?" && !event.metaKey && !event.ctrlKey) {
            event.preventDefault();
            showShortcuts();
            return;
        }
        const control = shortcutControls().find(function (element) { return parseShortcuts(element).some(function (shortcut) { return matchesShortcut(shortcut, event); }); });
        if (!control) return;
        event.preventDefault();
        if (isTyping(control)) {
            control.focus();
            if (control.select) control.select();
        } else control.click();
    });

    function isBack(activation) {
        return Boolean(activation && activation.from && activation.navigationType === "traverse" && activation.entry.index < activation.from.index);
    }

    function nameFor(transition, element, name) {
        if (!element) return;
        element.style.viewTransitionName = name;
        function clear() { element.style.viewTransitionName = ""; }
        transition.finished.then(clear, clear);
    }

    function findLink(url) {
        return Array.from(document.querySelectorAll(".list__row a[href], a.thumbnail[href], .thumbnail__link[href]")).find(function (link) { return link.href === url; });
    }

    function nameProfile(transition) {
        const profile = document.querySelector(".profile");
        if (!profile) return;
        nameFor(transition, profile, "detail");
        nameFor(transition, profile.querySelector(":scope > .avatar"), "detail-avatar");
    }

    function nameRow(transition, url) {
        const link = url && findLink(url);
        if (!link) return;
        const row = link.closest(".list__row, .thumbnail");
        nameFor(transition, row, "detail");
        nameFor(transition, row.querySelector(".avatar"), "detail-avatar");
    }

    addEventListener("pageswap", function (event) {
        if (!event.viewTransition || !event.activation) return;
        const back = isBack(event.activation);
        event.viewTransition.types.add(back ? "back" : "forward");
        if (back) nameProfile(event.viewTransition);
        else nameRow(event.viewTransition, event.activation.entry.url);
    });

    addEventListener("pagereveal", function (event) {
        const activation = window.navigation && navigation.activation;
        if (!event.viewTransition || !activation) return;
        const back = isBack(activation);
        event.viewTransition.types.add(back ? "back" : "forward");
        if (back) nameRow(event.viewTransition, activation.from && activation.from.url);
        else nameProfile(event.viewTransition);
    });

    function start() {
        statusIn(document.body);
        const existing = document.querySelector(".toast--floating");
        if (existing) {
            bindToast(existing);
            if (!existing.hidden && existing.textContent.trim()) {
                toastAction = Boolean(existing.querySelector(".toast__action"));
                announce(existing.textContent.trim());
                armToast();
            }
        }
        bindSearches(document);
        initTabs(document);
    }

    if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
    else start();

    window.design = {
        announce: announce,
        toast: function (message, action) { showToast(message, action); },
        hideToast: hideToast,
        sheet: openSheet,
        menu: openMenuAt,
        combobox: bindCombobox,
        bindSearches: bindSearches,
        focusFirst: focusFirst,
        focusNear: focusNear,
        stagger: stagger
    };
})();
