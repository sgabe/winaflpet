import { Collapse, Tooltip } from './bootstrap.bundle.min.js';

$(function () {

    $(document).ajaxStart(function(){
        $("#wait").attr("style", "display: flex !important");
    });

    $(document).ajaxComplete(function(){
        $("#wait").attr("style", "display: none !important");
    });

    document.querySelectorAll('[data-toggle="tooltip"]').forEach(function (element) {
        Tooltip.getOrCreateInstance(element);
    });

    setInterval(function(){
        $.ajax({
            url: "/user/refresh",
            method: "POST"
        })
    }, 5*60*1000);

    var getFilename = function(jqXHR) {
        var disposition = jqXHR.getResponseHeader("Content-Disposition");

        if (disposition && disposition.indexOf("attachment") !== -1) {
            var filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
            var matches = filenameRegex.exec(disposition);
            if (matches != null && matches[1]) {
                return matches[1].replace(/['"]/g, "");
            }
        }

        return "";
    }

    var showAlert = function(context, alert) {
        $(".alert").removeClass("alert-empty theme-danger theme-info").addClass("theme-" + context);
        $(".alert .message").html(alert);
        $("html, body").animate({ scrollTop: 0 }, "fast");
    }

    var showCheckResult = function(context, success, message) {
        var link = $(context).filter("a.action.check").get(0);
        var card = link ? link.closest(".card.agent, .agent-check-context") : null;
        var result = card ? card.querySelector(".agent-check-result") : null;
        var text = result ? result.querySelector(".agent-check-result-message") : null;
        if (!text) return;
        result.classList.toggle("is-success", success);
        result.classList.toggle("is-error", !success);
        result.hidden = false;
        text.textContent = message;
        var cards = document.getElementById("cards");
        if (cards && card.matches(".card.agent") && cards.contains(card) && $.fn.masonry) $(cards).masonry();
    }

    var toggleActions = function(context) {
        $(context).toggleClass("disabled");
        var group = $(context).closest(".card.job .card-actions").length
            ? $(context).closest(".card-actions").find(".action,.job-start,.job-stop,.job-download").not(context)
            : $(context).siblings();
        if (!$(context).is(".play")) {
            group.toggleClass("disabled");
        }
        else if ($(context).is(".last")) {
            group.filter(".disabled").not(".play").toggleClass("disabled");
        }
    }

    $("a.job-download,a.crash-report").click(async function(e) {
        e.preventDefault();
        var button = $(this);
        if (button.hasClass("disabled")) return;
        button.addClass("disabled");
        var report = button.hasClass("crash-report");
        if (report) $("#alert").removeClass("alert-empty theme-danger theme-success").addClass("theme-info").find(".message").text("Generating HTML report...");
        try {
            var response = await fetch(button.attr("href"), {method: button.attr("data-method") || "GET"});
            var type = response.headers.get("Content-Type") || "";
            if (type.includes("application/json")) {
                var status = await response.json();
                $("#alert").removeClass("alert-empty theme-danger theme-info theme-success")
                    .addClass(response.ok ? "theme-info" : "theme-danger").find(".message")
                    .text(status.alert || status.error || "Download failed.");
            } else {
                if (!response.ok || !type.includes(report ? "text/html" : "application/zip") || (report && !response.headers.get("Content-Disposition"))) throw new Error("Unable to download the file. Check your connection and login.");
                var blob = await response.blob();
                var url = URL.createObjectURL(blob);
                var link = document.createElement("a");
                link.href = url;
                link.download = getFilename({getResponseHeader: key => response.headers.get(key)}) || (report ? "report.html" : "crashes.zip");
                document.body.appendChild(link);
                link.click();
                link.remove();
                if (report) $("#alert").removeClass("theme-info theme-danger").addClass("theme-success").find(".message").text("HTML report downloaded.");
                setTimeout(() => URL.revokeObjectURL(url), 1000);
            }
        } catch (error) {
            $("#alert").removeClass("alert-empty theme-info theme-success").addClass("theme-danger").find(".message").text(error.message);
        } finally { button.removeClass("disabled"); }
    });

    var confirmDialog = document.getElementById("confirm");
    var confirmModal = !!(confirmDialog && typeof confirmDialog.showModal === "function");
    var confirmResolve = null;
    var confirmResult = false;
    var confirmTrigger = null;

    if (confirmModal) {
        confirmDialog.querySelector(".confirm-ok").addEventListener("click", function() {
            confirmResult = true;
            confirmDialog.close();
        });

        confirmDialog.querySelector(".confirm-cancel").addEventListener("click", function() {
            confirmDialog.close();
        });

        confirmDialog.addEventListener("click", function(e) {
            if (e.target === confirmDialog) confirmDialog.close();
        });

        confirmDialog.addEventListener("close", function() {
            var resolve = confirmResolve;
            var target = confirmTrigger;
            confirmResolve = null;
            confirmTrigger = null;
            if (target && target.isConnected && !target.getClientRects().length) {
                var more = target.closest(".card-actions-more");
                target = more ? more.querySelector('[data-bs-toggle="menu"]') : null;
            }
            if (target && target.isConnected && target.getClientRects().length) target.focus();
            if (resolve) resolve(confirmResult);
        });
    }

    var confirmAction = function(link) {
        var title = link.getAttribute("data-confirm-title") || "Delete?";
        var message = link.getAttribute("data-confirm-message") || title;
        if (!confirmModal) return Promise.resolve(window.confirm(message));
        if (confirmResolve || confirmDialog.open) return Promise.resolve(false);
        return new Promise(function(resolve) {
            confirmResolve = resolve;
            confirmResult = false;
            confirmTrigger = link;
            document.getElementById("confirm-title").textContent = title;
            document.getElementById("confirm-message").textContent = message;
            confirmDialog.showModal();
            confirmDialog.querySelector(".confirm-cancel").focus();
        });
    }

    $("a.action").click(async function(e) {
        var isCustom = $(this).attr("data-method");
        var isDisabled = $(this).is("a.disabled");

        if (isCustom || isDisabled) {
            e.preventDefault();
        }

        if (isCustom && !isDisabled) {
            if (String(isCustom).toUpperCase() === "DELETE") {
                if (!(await confirmAction(this))) return;
                if (!this.isConnected || $(this).is("a.disabled")) return;
            }
            var link = this;
            var isCheck = $(link).is("a.action.check") && !!link.closest(".card.agent, .agent-check-context");
            var ariaDisabled = link.getAttribute("aria-disabled");
            if (isCheck) {
                link.classList.add("disabled");
                link.setAttribute("aria-disabled", "true");
                link.setAttribute("aria-busy", "true");
            }
            $.ajax({
                url: $(this).attr('href'),
                method: $(this).attr("data-method"),
                context: $(this),
                dataType: $(this).attr("data-type") ? "binary" : "json",
                global: !isCheck,
                statusCode: {
                    401: function() {
                        setTimeout(function() {
                            window.location.href = "/user/login";
                        }, 1000*2)
                    }
                },
            }).done(function(data, textStatus, jqXHR ) {
                if (isCheck) {
                    if (data !== null && typeof data === "object" && typeof data.context === "string" && typeof data.alert === "string") {
                        showCheckResult(link, data.context.includes("success"), data.alert);
                    } else {
                        showCheckResult(link, false, "The server returned an unexpected response while checking the connection.");
                    }
                    return;
                }
                if (data.hasOwnProperty("context") && data.hasOwnProperty("alert")) {
                    showAlert(data.context, data.alert);
                    if (data.context.includes("success")) {
                        toggleActions(this);
                        if ($(this).is("a.plot")) {
                            var context = $(this);
                            setTimeout(function() {
                                window.location.href = context.attr('href');
                            }, 1000*5);
                        } else if ($(this).is("a.verify") || $(this).is("a.play") || $(this).is("a.delete")) {
                            setTimeout(function() {
                                location.reload()
                            }, 1000*5);
                        }
                    }
                } else {
                    var blob = new Blob([data], {type: jqXHR.getResponseHeader("Content-Type")});
                    var URL = window.URL || window.webkitURL;
                    var downloadUrl = URL.createObjectURL(blob);

                    var filename = getFilename(jqXHR);
                    if (filename) {
                        var a = document.createElement("a");
                        a.href = downloadUrl;
                        a.download = filename;
                        document.body.appendChild(a);
                        a.click();
                    } else {
                        window.location = downloadUrl;
                    }

                    setTimeout(function () { URL.revokeObjectURL(downloadUrl); }, 100);
                }
            }).fail(function(jqXHR, textStatus) {
                var json = jqXHR && jqXHR.responseJSON;
                if (isCheck) {
                    showCheckResult(link, false, json && typeof json.alert === "string" ? json.alert
                        : jqXHR && jqXHR.status === 401 ? "You are not logged in. Please log in and try again."
                        : "Connection check failed. Check your connection and try again.");
                    return;
                }
                showAlert("danger", "You are not logged in. Please log in and try again.");
            }).always(function() {
                if (!isCheck) return;
                link.classList.remove("disabled");
                link.removeAttribute("aria-busy");
                if (ariaDisabled === null) link.removeAttribute("aria-disabled");
                else link.setAttribute("aria-disabled", ariaDisabled);
            });
        }
    });

    $(".close").on("click", function() {
        $(".alert").toggleClass("alert-empty");
    });

    initPlotRefresh();

    $("form.create").submit(function(e) {
        e.preventDefault();

        $.ajax({
            url: $(this).attr("action"),
            method: "PUT",
            data: $(this).serialize(),
            context: $(this),
        }).done(function(data) {
            showAlert(data.context, data.alert);
        });
    });

    $("#autoresume").change(function() {
        if($(this).is(":checked")) {
            $("#skipCrashes").prop("checked", true);
        }
    });

    var cards = document.getElementById("cards");
    if (cards && $.fn.masonry) {
        $(cards).masonry({
            itemSelector: '.col'
        });
        ["shown.bs.collapse", "hidden.bs.collapse"].forEach(function(name) {
            cards.addEventListener(name, function() {
                $(cards).masonry();
            });
        });
    }

});

$.ajaxTransport("+binary", function(options, originalOptions, jqXHR) {
    var isBinary = options.dataType && options.dataType == "binary",
        isBlob = options.data && window.Blob && options.data instanceof Blob,
        isArrayBuffer = options.data && window.ArrayBuffer && options.data instanceof ArrayBuffer;
    if (window.FormData && (isBinary || isArrayBuffer || isBlob)) {
        return {
            send: function(headers, callback) {
                var xhr = new XMLHttpRequest(),
                    url = options.url,
                    type = options.type,
                    async = options.async || true,
                    dataType = options.responseType || "blob",
                    data = options.data || null;

                xhr.addEventListener("load", function() {
                    var data = {};
                    data[options.dataType] = xhr.response;
                    callback(xhr.status, xhr.statusText, data, xhr.getAllResponseHeaders());
                });

                xhr.open(type, url, async);
                xhr.responseType = dataType;
                xhr.send(data);
            },
            abort: function() {
                jqXHR.abort();
            }
        };
    }
});

jQuery.expr.pseudos.contains = jQuery.expr.createPseudo(function(text) {
    var query = String(text).toUpperCase();
    return function(element) {
        return jQuery(element).text().toUpperCase().indexOf(query) !== -1;
    };
});

var filterCardsEmpty = null;
var filterCardsConfig = "all";
var filterCardsToolbar = null;
var filterCardsAgent = "all";
var filterCardsJob = "all";
var filterCardsRisk = "all";
var filterCardsFunction = "all";

function cardListAgent(column) {
    var job = column.querySelector(".card.job");
    var id = job ? job.getAttribute("data-agent-id") : "";
    return id ? String(id) : "";
}

function cardListJob(column) {
    var crash = column.querySelector(".card.crash");
    var id = crash ? crash.getAttribute("data-job-id") : "";
    return id ? String(id) : "";
}

function cardListRisk(column) {
    var crash = column.querySelector(".card.crash");
    if (!crash || crash.classList.contains("unverified")) return "na";
    var risk = crash.getAttribute("data-risk");
    return ["high", "medium", "low", "none", "na"].indexOf(risk) >= 0 ? risk : "na";
}

function cardListFunction(column) {
    var crash = column.querySelector(".card.crash");
    var name = crash ? crash.getAttribute("data-function") : "";
    return name ? String(name) : "";
}

function cardListFunctionMatch(column, selected) {
    if (selected === "all") return true;
    var name = cardListFunction(column);
    if (selected === "missing") return name === "";
    return name !== "" && selected === "fn:" + name;
}

function cardListSelect(definition) {
    var controlId = definition.prefix;
    for (var n = 2; document.getElementById(controlId); n++) {
        controlId = definition.prefix + "-" + n;
    }
    var group = document.getElementById("tpl-card-list-select").content.firstElementChild.cloneNode(true);
    group.classList.add(definition.className);
    var label = group.querySelector("label");
    label.setAttribute("for", controlId);
    label.textContent = definition.label;
    var select = group.querySelector("select");
    select.id = controlId;
    select.appendChild(new Option(definition.allLabel, "all"));
    definition.items.forEach(function(item) {
        select.appendChild(new Option(item.label, item.id));
    });
    select.addEventListener("change", function() {
        definition.change(select.value);
    });
    return group;
}

function cardListConfig(column) {
    if (column.querySelector(".card.agent .card-summary-badge.is-enabled")) return "enabled";
    if (column.querySelector(".card.agent .card-summary-badge.is-disabled")) return "disabled";
    var crash = column.querySelector(".card.crash");
    if (crash) {
        if (crash.classList.contains("unverified")) return "unverified";
        if (crash.classList.contains("unavailable")) return "unavailable";
        return "verified";
    }
    var job = column.querySelector(".card.job");
    if (job) {
        var status = job.getAttribute("data-list-status");
        return ["running", "starting", "stopped"].indexOf(status) >= 0 ? status : "unknown";
    }
    return "";
}

function cardListCounts(selector) {
    var cards = document.getElementById("cards");
    var counts = { total: 0 };
    if (!cards) return counts;
    Array.prototype.forEach.call(cards.children, function(child) {
        if (!child.classList.contains("col") || !child.querySelector(selector)) return;
        var key = cardListConfig(child);
        counts.total++;
        counts[key] = (counts[key] || 0) + 1;
    });
    return counts;
}

function updateCardListToolbar() {
    var cards = document.getElementById("cards");
    if (!filterCardsToolbar || !cards) return;
    var counts = {};
    filterCardsToolbar.buttons.forEach(function(button) {
        button.setAttribute("aria-pressed", String(button.filterValue === filterCardsConfig));
        if (!counts[button.filterSelector]) counts[button.filterSelector] = cardListCounts(button.filterSelector);
        var value = String(counts[button.filterSelector][button.filterValue === "all" ? "total" : button.filterValue] || 0);
        if (button.filterCount.textContent !== value) button.filterCount.textContent = value;
    });
}

function cardListFiltered() {
    var search = document.getElementById("search");
    return !!(search && String(search.value).trim()) || [filterCardsConfig, filterCardsAgent, filterCardsJob, filterCardsRisk, filterCardsFunction].some(function(value) {
        return value !== "all";
    });
}

function clearCardFilters() {
    var search = document.getElementById("search");
    if (search) search.value = "";
    filterCardsConfig = filterCardsAgent = filterCardsJob = filterCardsRisk = filterCardsFunction = "all";
    if (filterCardsToolbar) {
        filterCardsToolbar.querySelectorAll("select").forEach(function(select) {
            select.value = "all";
        });
    }
    var target = search && search.getClientRects().length ? search : filterCardsToolbar && filterCardsToolbar.querySelector("button, select");
    if (target) target.focus();
    filterCards();
}

var filterCards = function(initial) {
    var cards = document.getElementById("cards");
    if (!cards) return;

    var search = document.getElementById("search");
    var query = (search && search.value ? String(search.value) : "").trim().toLowerCase();
    var columns = Array.prototype.filter.call(cards.children, function(child) {
        return child.classList.contains("col");
    });

    var visible = 0;
    columns.forEach(function(column) {
        var match = (filterCardsConfig === "all" || cardListConfig(column) === filterCardsConfig) && (filterCardsAgent === "all" || cardListAgent(column) === filterCardsAgent) && (filterCardsJob === "all" || cardListJob(column) === filterCardsJob) && (filterCardsRisk === "all" || cardListRisk(column) === filterCardsRisk) && cardListFunctionMatch(column, filterCardsFunction) && (!query || Array.prototype.some.call(
            column.querySelectorAll(".card-header, .card-body"),
            function(part) {
                return String(part.textContent).toLowerCase().indexOf(query) !== -1;
            }
        ));
        column.classList.toggle("d-none", !match);
        if (match) visible++;
    });

    if (columns.length && !filterCardsEmpty && cards.parentNode) {
        var noun = cards.querySelector(".card.agent") ? "agents" : cards.querySelector(".card.job") ? "jobs" : cards.querySelector(".card.crash") ? "crashes" : "cards";
        filterCardsEmpty = document.getElementById("tpl-card-list-empty").content.firstElementChild.cloneNode(true);
        filterCardsEmpty.querySelector("h2").textContent = "No matching " + noun;
        filterCardsEmpty.querySelector("p").textContent = "No " + noun + " on this page match the current search and filters.";
        filterCardsEmpty.querySelector("button").addEventListener("click", clearCardFilters);
        cards.parentNode.insertBefore(filterCardsEmpty, cards.nextSibling);
    }
    var filtered = cardListFiltered();
    if (filterCardsEmpty) {
        filterCardsEmpty.hidden = !(filtered && columns.length && visible === 0);
    }

    updateCardFoldTools();
    updateCardListToolbar();

    if (initial && !filtered) return;
    if ($.fn.masonry) $(cards).masonry();
}

$(window).on('load', function() {
    filterCards(true);
})

$(function () {
    var stats = document.querySelector("#content.job-stats");
    if (!stats) return;

    ["shown.bs.collapse", "hidden.bs.collapse"].forEach(function(name) {
        stats.addEventListener(name, function(e) {
            var trigger = stats.querySelector('[aria-controls="' + e.target.id + '"]');
            if (trigger) trigger.setAttribute("aria-expanded", String(e.target.classList.contains("show")));
        });
    });

    var panels = Array.prototype.slice.call(stats.querySelectorAll(".card.fuzzer"));
    var toolbar = null;

    var overview = document.querySelector(".job-overview");
    var cores = String(overview ? overview.getAttribute("data-cores") || "" : "").trim();
    cores = /^\d+$/.test(cores) ? Number(cores) : 0;
    if (!Number.isSafeInteger(cores) || cores > 4096) cores = 0;
    var banner = String(overview ? overview.getAttribute("data-banner") || "" : "").replace(/\s+/g, " ").trim();
    var fids = panels.map(function(panel) {
        var fid = String(panel.getAttribute("data-fid") || "").trim();
        return /^\d+$/.test(fid) ? Number(fid) : 0;
    });
    var configured = cores > 0 && panels.length > 0 && fids.every(function(fid, index) {
        return Number.isSafeInteger(fid) && fid >= 1 && fid <= cores && fids.indexOf(fid) === index;
    });
    var total = configured ? cores : panels.length;

    if (panels.length) {
        var definitions = [];
        var checkboxes = [];
        var showDefinitions = function(show) {
            definitions.forEach(function(definition) {
                definition.hidden = !show;
            });
            checkboxes.forEach(function(checkbox) {
                checkbox.checked = show;
            });
        };
        panels.forEach(function(panel) {
            panel.querySelectorAll(".job-stat-count").forEach(function(count) {
                var text = String(count.textContent).trim();
                if (/^\d+$/.test(text)) count.textContent = text.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
            });
            panel.querySelectorAll('[data-toggle="tooltip"][title], [data-toggle="tooltip"][data-bs-original-title]').forEach(function(label) {
                var item = label.closest("li.list-group-item");
                if (!item || !panel.contains(item)) return;
                var text = (label.getAttribute("title") || label.getAttribute("data-bs-original-title") || "").replace(/\s+/g, " ").trim();
                if (!text || label.jobStatDefinition) return;
                var definition = document.createElement("span");
                definition.className = "job-stat-definition";
                definition.hidden = false;
                definition.textContent = text;
                label.jobStatDefinition = definition;
                item.appendChild(definition);
                definitions.push(definition);
            });

            var header = panel.querySelector(":scope > .card-header");
            if (!header) return;
            var group = document.createElement("div");
            group.className = "job-instance-actions";
            var toggle = document.createElement("label");
            toggle.className = "job-definitions-toggle";
            var checkbox = document.createElement("input");
            checkbox.type = "checkbox";
            checkbox.checked = true;
            checkbox.addEventListener("change", function() {
                showDefinitions(checkbox.checked);
            });
            toggle.appendChild(checkbox);
            toggle.appendChild(document.createTextNode("Show definitions"));
            group.appendChild(toggle);
            checkboxes.push(checkbox);
            var footer = panel.querySelector(":scope > .card-footer");
            var actions = footer ? footer.querySelector(".card-actions") : null;
            if (actions) {
                group.appendChild(actions);
                if (!footer.children.length) footer.hidden = true;
            }
            header.appendChild(group);
        });

        toolbar = document.createElement("div");
        toolbar.className = "job-stats-toolbar";
        var toolbarLabel = document.createElement("span");
        toolbarLabel.className = "job-stats-toolbar-label";
        toolbarLabel.textContent = configured
            ? "Fuzzer instances (" + panels.length + " of " + cores + " with statistics)"
            : "Fuzzer instances (" + panels.length + ")";
        toolbar.appendChild(toolbarLabel);
        if (total > 1) {
            var hint = document.createElement("span");
            hint.className = "job-stats-toolbar-hint";
            hint.textContent = configured && panels.length < cores
                ? "Select an instance to see its statistics. Dashed tiles are instances with no statistics received and can't be selected."
                : "Select an instance to see its statistics.";
            toolbar.appendChild(hint);
        }

        var accordion = stats.querySelector(".accordion");
        var anchor = accordion && accordion.parentNode ? accordion : panels[0];
        anchor.parentNode.insertBefore(toolbar, anchor);
    }

    if (!panels.length) return;

    var picker = document.createElement("div");
    picker.className = "job-instance-picker";
    picker.setAttribute("role", "group");
    picker.setAttribute("aria-label", "Fuzzer instances");

    var select = function(target) {
        panels.forEach(function(panel) {
            panel.hidden = panel !== target;
        });
        available.forEach(function(tile) {
            tile.button.setAttribute("aria-pressed", String(tile.panel === target));
            tile.button.tabIndex = tile.panel === target ? 0 : -1;
        });
    };

    var entries = [];
    panels.forEach(function(panel, index) {
        if (!panel.id) panel.id = "job-instance-" + index;
    });
    if (configured) {
        for (var fid = 1; fid <= cores; fid++) {
            var at = fids.indexOf(fid);
            entries.push({ number: fid, panel: at >= 0 ? panels[at] : null });
        }
    } else {
        panels.forEach(function(panel, index) {
            var number = fids[index];
            if (!Number.isSafeInteger(number) || number < 1) number = index + 1;
            entries.push({ number: number, panel: panel });
        });
    }

    var tiles = entries.map(function(entry) {
        var button = document.createElement("button");
        var tile = { button: button, panel: entry.panel };
        button.type = "button";
        button.className = "job-instance-tile";
        if (configured) {
            var mark = document.createElement("span");
            mark.className = "job-instance-tile-mark";
            mark.setAttribute("aria-hidden", "true");
            button.appendChild(mark);
        }
        button.appendChild(document.createTextNode(String(entry.number)));
        if (!entry.panel) {
            var missing = (banner || "Instance ") + entry.number + ": No statistics received";
            button.classList.add("is-missing");
            button.disabled = true;
            button.title = missing;
            button.setAttribute("aria-label", missing);
            picker.appendChild(button);
            return tile;
        }
        var name = entry.panel.querySelector(".job-instance-name");
        var label = (name ? name.textContent : "").replace(/\s+/g, " ").trim();
        if (configured) {
            button.classList.add("is-available");
            label = (label || (banner || "Instance ") + entry.number) + ": Statistics available";
        }
        button.title = label;
        button.setAttribute("aria-label", "Instance " + entry.number + ": " + label);
        button.setAttribute("aria-controls", entry.panel.id);
        button.addEventListener("click", function() {
            select(entry.panel);
        });
        button.addEventListener("keydown", function(e) {
            if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
            var index = available.indexOf(tile);
            var count = available.length;
            var target;
            if (e.key === "ArrowRight") target = (index + 1) % count;
            else if (e.key === "ArrowLeft") target = (index + count - 1) % count;
            else if (e.key === "Home") target = 0;
            else if (e.key === "End") target = count - 1;
            else return;
            e.preventDefault();
            select(available[target].panel);
            available[target].button.focus();
        });
        picker.appendChild(button);
        return tile;
    });
    var available = tiles.filter(function(tile) {
        return tile.panel;
    });

    if (toolbar && toolbar.parentNode) {
        toolbar.parentNode.insertBefore(picker, toolbar.nextSibling);
    } else {
        stats.insertBefore(picker, stats.firstChild);
    }
    select(panels[0]);
});

$(function () {
    var forms = Array.prototype.slice.call(document.querySelectorAll("form.settings-panel"));

    forms.forEach(function(form, formIndex) {
        var previous = form.previousElementSibling;
        if (previous && previous.matches("nav.settings-navigation")) return;

        var sections = Array.prototype.filter.call(form.children, function(child) {
            return child.matches("fieldset.settings-section") && Array.prototype.some.call(child.children, function(part) {
                return part.tagName === "LEGEND";
            });
        });
        if (sections.length < 4 || !form.parentNode) return;

        var prefix = form.id || "settings-form-" + (formIndex + 1);
        var jobForm = form.id === "createJob" || form.id === "editJob";
        var nav = document.createElement("nav");
        nav.className = "settings-navigation";
        nav.setAttribute("aria-label", "Form sections");

        var label = document.createElement("small");
        label.className = "settings-navigation-label";
        label.textContent = "Jump to section";
        nav.appendChild(label);

        var links = [];
        var advanced = null;
        var sectionIcons = {
            "General": "info-circle",
            "Tool locations": "wrench",
            "Input & output": "folder-open",
            "Instrumentation & target": "crosshairs",
            "Advanced": "cog"
        };
        var sectionFields = function(section) {
            return Array.prototype.filter.call(section.querySelectorAll("input, select, textarea"), function(control) {
                return control.tagName !== "INPUT" || ["hidden", "button", "submit", "reset", "image"].indexOf(control.type) < 0;
            }).length;
        };
        var frame = 0;
        var schedule = function() {
            if (!frame) frame = window.requestAnimationFrame(updateCurrent);
        };
        var locationHash = function() {
            try {
                return decodeURIComponent(window.location.hash.slice(1));
            } catch (error) {
                return window.location.hash.slice(1);
            }
        };
        var updateCurrent = function() {
            frame = 0;
            var root = document.documentElement;
            var hash = locationHash();
            var active = 0;
            var target = -1;
            sections.forEach(function(section, index) {
                var top = section.getBoundingClientRect().top;
                var margin = parseFloat(window.getComputedStyle(section).scrollMarginTop) || 0;
                if (top <= Math.max(100, margin) + 1) active = index;
                if (section.id === hash && top >= 0 && top < window.innerHeight) target = index;
            });
            var scrollable = root.scrollHeight > window.innerHeight + 2;
            var bottom = scrollable && window.innerHeight + window.scrollY >= root.scrollHeight - 2;
            if (target >= 0 && (bottom || !scrollable)) active = target;
            else if (bottom) active = sections.length - 1;
            links.forEach(function(link, index) {
                if (index === active) link.setAttribute("aria-current", "location");
                else link.removeAttribute("aria-current");
            });
        };

        sections.forEach(function(section, sectionIndex) {
            var legend = Array.prototype.find.call(section.children, function(part) {
                return part.tagName === "LEGEND";
            });
            if (!section.id) {
                var base = prefix + "-section-" + (sectionIndex + 1);
                var id = base;
                var suffix = 2;
                while (document.getElementById(id)) {
                    id = base + "-" + suffix;
                    suffix++;
                }
                section.id = id;
            }

            var link = document.createElement("a");
            link.className = "settings-navigation-link";
            link.setAttribute("href", "#" + section.id);
            var text = String(legend.textContent).replace(/\s+/g, " ").trim() || "Section " + (sectionIndex + 1);
            if (jobForm) {
                if (Object.prototype.hasOwnProperty.call(sectionIcons, text)) {
                    var icon = document.createElement("i");
                    icon.className = "fas fa-" + sectionIcons[text] + " settings-navigation-icon";
                    icon.setAttribute("aria-hidden", "true");
                    link.appendChild(icon);
                }
                var name = document.createElement("span");
                name.className = "settings-navigation-text";
                name.textContent = text;
                var count = document.createElement("span");
                count.className = "settings-navigation-count";
                if (text === "Advanced") advanced = { section: section, count: count };
                else count.textContent = String(sectionFields(section));
                link.appendChild(name);
                link.appendChild(document.createTextNode(" "));
                link.appendChild(count);
            } else {
                link.textContent = text;
            }
            nav.appendChild(link);
            links.push(link);

            if (!jobForm) return;
            if (!legend.hasAttribute("tabindex")) legend.setAttribute("tabindex", "-1");
            link.addEventListener("click", function(e) {
                if (e.defaultPrevented || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
                setTimeout(function() {
                    if (legend.isConnected) legend.focus({ preventScroll: true });
                    schedule();
                }, 0);
            });
        });

        if (jobForm) {
            var required = 0;
            Array.prototype.forEach.call(form.elements, function(control) {
                if (!control.hasAttribute("required") || !control.labels) return;
                Array.prototype.forEach.call(control.labels, function(text) {
                    if (text.querySelector(".form-required-marker")) return;
                    var marker = document.createElement("span");
                    marker.className = "form-required-marker";
                    marker.setAttribute("aria-hidden", "true");
                    marker.textContent = " *";
                    text.appendChild(marker);
                });
                required++;
            });
            if (required) {
                var note = document.createElement("p");
                note.className = "settings-navigation-note";
                note.textContent = "Fields marked * are required.";
                nav.appendChild(note);
            }

            if (!form.querySelector(":scope > .job-form-intro")) {
                var intro = document.createElement("p");
                intro.className = "form-notice job-form-intro";
                intro.textContent = "Paths refer to the selected agent machine. Placeholder examples are hints, not saved values.";
                form.insertBefore(intro, sections[0]);
            }

            Array.prototype.forEach.call(form.querySelectorAll('input[type="text"]'), function(input) {
                if (input.name === "name" || input.name === "desc") return;
                input.setAttribute("spellcheck", "false");
            });

            if (advanced) {
                var countChecked = function() {
                    advanced.count.textContent = advanced.section.querySelectorAll('input[type="checkbox"]:checked').length + " on";
                };
                advanced.section.addEventListener("change", function(e) {
                    if (e.target.type === "checkbox") countChecked();
                });
                countChecked();
            }

            window.addEventListener("scroll", schedule, { passive: true });
            window.addEventListener("resize", schedule, { passive: true });
            window.addEventListener("hashchange", schedule);
            window.addEventListener("load", schedule);
        }

        form.parentNode.insertBefore(nav, form);
        if (form.parentNode.classList.contains("settings-form-layout")) {
            form.parentNode.classList.add("has-section-navigation");
        }
        if (jobForm) {
            updateCurrent();
            var initialHash = locationHash();
            var initial = initialHash ? sections.filter(function(section) {
                return section.id === initialHash;
            })[0] : null;
            if (initial) {
                var reveal = function() {
                    window.requestAnimationFrame(function() {
                        if (!initial.isConnected || locationHash() !== initialHash || Math.abs(window.scrollY) > 1) return;
                        initial.scrollIntoView({ block: "start" });
                        updateCurrent();
                    });
                };
                if (document.readyState === "complete") reveal();
                else window.addEventListener("load", reveal, { once: true });
            }
        }
    });
});

function cardFoldItems(cards) {
    var items = [];
    Array.prototype.forEach.call(cards.children, function(column) {
        if (!column.classList.contains("col")) return;
        var toggle = column.querySelector(".card-summary .card-toggle");
        if (!toggle) return;
        var target = toggle.getAttribute("aria-controls") || toggle.getAttribute("data-bs-target") || toggle.getAttribute("data-target") || toggle.getAttribute("href") || "";
        var id = target.replace(/^#/, "");
        if (!id) return;
        var panel = Array.prototype.filter.call(column.querySelectorAll(".collapse"), function(candidate) {
            return candidate.id === id;
        })[0];
        if (!panel) return;
        for (var parent = panel.parentElement; parent && parent !== column; parent = parent.parentElement) {
            if (parent.classList.contains("collapse")) return;
        }
        items.push({
            column: column,
            panel: panel,
            visible: !column.classList.contains("d-none") && !column.hidden
        });
    });
    return items;
}

function updateCardFoldTools() {
    var tools = document.querySelector(".card-fold-tools");
    var cards = document.getElementById("cards");
    if (!tools || !cards) return;
    var button = tools.querySelector("button");
    if (!button) return;
    var panels = cardFoldItems(cards).filter(function(item) {
        return item.visible;
    }).map(function(item) {
        return item.panel;
    });
    if (panels.some(function(panel) { return panel.classList.contains("collapsing"); })) {
        button.disabled = true;
        return;
    }
    var open = panels.length > 0 && panels.every(function(panel) {
        return panel.classList.contains("show");
    });
    button.disabled = panels.length === 0;
    button.textContent = open ? "Collapse all" : "Expand all";
    button.setAttribute("aria-label", open ? "Collapse all shown cards" : "Expand all shown cards");
    button.setAttribute("aria-pressed", String(open));
}

$(function () {
    var cards = document.getElementById("cards");
    if (!cards || !cards.parentNode || filterCardsToolbar) return;
    var columns = Array.prototype.filter.call(cards.children, function(child) {
        return child.classList.contains("col");
    });
    if (!columns.length) return;

    var source = document.getElementById("tpl-card-list-toolbar");
    if (!source) return;
    var toolbar = source.content.firstElementChild.cloneNode(true);
    var noun = cards.querySelector(".card.agent") ? "agents" : cards.querySelector(".card.job") ? "jobs" : "crashes";
    toolbar.setAttribute("aria-label", "Filter " + noun + " on this page");
    if (noun === "crashes") toolbar.querySelector("#card-list-scope").textContent += " Unavailable crashes were verified but have no reported function.";
    var search = toolbar.querySelector("input");
    search.placeholder = "Search " + noun;
    search.setAttribute("aria-label", "Search " + noun);
    search.addEventListener("input", function() { filterCards(); });
    var selects = toolbar.querySelector(".card-list-selects");
    var buttons = [];
    [
        [".card.agent", "Agent configuration", [["All", "all"], ["Enabled", "enabled"], ["Disabled", "disabled"]]],
        [".card.crash", "Crash verification", [["All", "all"], ["Verified", "verified"], ["Unverified", "unverified"], ["Unavailable", "unavailable"]]],
        [".card.job", "Job status", [["All", "all"], ["Running", "running"], ["Starting", "starting"], ["Stopped", "stopped"], ["Unknown", "unknown"]]]
    ].forEach(function(definition) {
        if (!columns.some(function(column) { return column.querySelector(definition[0]); })) return;
        var group = toolbar.querySelector(".card-list-filter");
        group.setAttribute("aria-label", definition[1]);
        definition[2].forEach(function(option) {
            var button = document.getElementById("tpl-card-list-status").content.firstElementChild.cloneNode(true);
            button.querySelector(".card-list-filter-label").textContent = option[0];
            button.setAttribute("data-status", option[1]);
            var tally = button.querySelector(".card-list-filter-count");
            button.filterValue = option[1];
            button.filterSelector = definition[0];
            button.filterCount = tally;
            button.addEventListener("click", function() {
                filterCardsConfig = option[1];
                filterCards();
            });
            group.appendChild(button);
            buttons.push(button);
        });
    });

    var agents = [];
    columns.forEach(function(column) {
        var id = cardListAgent(column);
        if (!id || agents.some(function(agent) { return agent.id === id; })) return;
        var host = column.querySelector(".card.job .card-summary .host");
        var name = host ? String(host.textContent).replace(/\s+/g, " ").trim() : "";
        agents.push({ id: id, label: name ? name + " (#" + id + ")" : "Agent #" + id });
    });
    if (agents.length) {
        selects.appendChild(cardListSelect({
            prefix: "card-list-agent-select",
            className: "card-list-agent",
            label: "Agent",
            allLabel: "All agents",
            items: agents,
            change: function(value) {
                filterCardsAgent = value;
                filterCards();
            }
        }));
    }

    var crashJobs = [];
    columns.forEach(function(column) {
        var id = cardListJob(column);
        if (!id || crashJobs.some(function(job) { return job.id === id; })) return;
        var summaryLabel = column.querySelector(".card.crash .card-summary-label");
        var name = summaryLabel ? String(summaryLabel.textContent).replace(/\s+/g, " ").trim() : "";
        crashJobs.push({ id: id, label: name ? name + " (#" + id + ")" : "Job #" + id });
    });
    if (crashJobs.length) {
        selects.appendChild(cardListSelect({
            prefix: "card-list-job-select",
            className: "card-list-job",
            label: "Job",
            allLabel: "All jobs",
            items: crashJobs,
            change: function(value) {
                filterCardsJob = value;
                filterCards();
            }
        }));
    }

    if (columns.some(function(column) { return column.querySelector(".card.crash"); })) {
        selects.appendChild(cardListSelect({
            prefix: "card-list-risk-select",
            className: "card-list-risk",
            label: "Crash Risk",
            allLabel: "All risks",
            items: [
                { id: "high", label: "High" },
                { id: "medium", label: "Medium" },
                { id: "low", label: "Low" },
                { id: "none", label: "None" },
                { id: "na", label: "Not assessed" }
            ],
            change: function(value) {
                filterCardsRisk = value;
                filterCards();
            }
        }));
    }

    if (columns.some(function(column) { return column.querySelector(".card.crash"); })) {
        var functionNames = [];
        var functionMissing = false;
        columns.forEach(function(column) {
            if (!column.querySelector(".card.crash")) return;
            var name = cardListFunction(column);
            if (!name) functionMissing = true;
            else if (functionNames.indexOf(name) < 0) functionNames.push(name);
        });
        functionNames.sort(function(a, b) {
            return a < b ? -1 : a > b ? 1 : 0;
        });
        var functionItems = functionNames.map(function(name) {
            return { id: "fn:" + name, label: name };
        });
        if (functionMissing) functionItems.push({ id: "missing", label: "No recorded function" });
        selects.appendChild(cardListSelect({
            prefix: "card-list-function-select",
            className: "card-list-function",
            label: "Function",
            allLabel: "All functions",
            items: functionItems,
            change: function(value) {
                filterCardsFunction = value;
                filterCards();
            }
        }));
    }

    if (!selects.children.length) selects.remove();
    toolbar.buttons = buttons;
    filterCardsToolbar = toolbar;
    cards.parentNode.insertBefore(toolbar, cards);
    updateCardListToolbar();
});

$(function () {
    var cards = document.getElementById("cards");
    if (!cards || !cards.parentNode) return;
    if (!filterCardsToolbar || filterCardsToolbar.parentNode !== cards.parentNode) return;
    if (!cardFoldItems(cards).length) return;

    var tools = filterCardsToolbar.querySelector(".card-fold-tools");
    var button = tools.querySelector("button");
    button.addEventListener("click", function() {
        var items = cardFoldItems(cards).filter(function(item) {
            return item.visible;
        });
        if (!items.length || items.some(function(item) { return item.panel.classList.contains("collapsing"); })) {
            updateCardFoldTools();
            return;
        }
        var open = items.every(function(item) {
            return item.panel.classList.contains("show");
        });
        items.forEach(function(item) {
            var instance = Collapse.getOrCreateInstance(item.panel, { toggle: false });
            if (open) instance.hide();
            else instance.show();
        });
        updateCardFoldTools();
    });

    ["shown.bs.collapse", "hidden.bs.collapse"].forEach(function(name) {
        cards.addEventListener(name, updateCardFoldTools);
    });

    updateCardFoldTools();
});

function initSecretToggles() {
    Array.prototype.forEach.call(document.querySelectorAll("[data-secret-toggle]"), function(button) {
        if (button.dataset.secretBound) return;
        var target = document.getElementById(button.getAttribute("data-secret-toggle"));
        if (!target || target.tagName !== "INPUT" || (target.type !== "password" && target.type !== "text")) return;
        button.dataset.secretBound = "1";
        button.addEventListener("click", function() {
            var show = target.type === "password";
            target.type = show ? "text" : "password";
            button.setAttribute("aria-pressed", String(show));
            button.textContent = show ? "Hide" : "Show";
            button.setAttribute("aria-label", show ? "Hide API key" : "Show API key");
        });
    });
}

$(initSecretToggles);

function formatFileSize(bytes) {
    if (!(bytes >= 1024)) return String(bytes || 0) + " bytes";
    var units = ["KiB", "MiB", "GiB"];
    var value = bytes / 1024;
    var unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit++;
    }
    return String(Math.round(value * 10) / 10) + " " + units[unit];
}

function initFileSummaries() {
    Array.prototype.forEach.call(document.querySelectorAll("[data-file-summary]"), function(input) {
        if (input.dataset.fileSummaryBound) return;
        var output = document.getElementById(input.getAttribute("data-file-summary"));
        if (!output) return;
        input.dataset.fileSummaryBound = "1";
        var update = function() {
            var file = input.files && input.files.length ? input.files[0] : null;
            if (!file) {
                output.textContent = "";
                output.hidden = true;
                return;
            }
            output.textContent = "Selected: " + file.name + " (" + formatFileSize(file.size) + ")";
            output.hidden = false;
        };
        input.addEventListener("change", update);
        update();
    });
}

$(initFileSummaries);

function initFileDrops() {
    Array.prototype.forEach.call(document.querySelectorAll("[data-file-drop]"), function(zone) {
        if (zone.dataset.fileDropBound) return;
        var input = document.getElementById(zone.getAttribute("data-file-drop"));
        if (!input || input.tagName !== "INPUT" || input.type !== "file" || !zone.contains(input)) return;
        var status = document.getElementById(zone.getAttribute("data-file-drop-status") || "");
        zone.dataset.fileDropBound = "1";
        var depth = 0;

        var hasFiles = function(e) {
            var types = e.dataTransfer && e.dataTransfer.types;
            return !!types && Array.prototype.indexOf.call(types, "Files") >= 0;
        };

        var show = function(text) {
            if (!status) return;
            status.textContent = text;
            status.hidden = !text;
        };

        var reset = function() {
            depth = 0;
            zone.classList.remove("is-dragover");
        };

        zone.addEventListener("dragenter", function(e) {
            if (!hasFiles(e)) return;
            e.preventDefault();
            if (input.disabled) {
                e.dataTransfer.dropEffect = "none";
                reset();
                return;
            }
            depth++;
            zone.classList.add("is-dragover");
        });

        zone.addEventListener("dragover", function(e) {
            if (!hasFiles(e)) return;
            e.preventDefault();
            if (input.disabled) {
                e.dataTransfer.dropEffect = "none";
                reset();
                return;
            }
            e.dataTransfer.dropEffect = "copy";
            zone.classList.add("is-dragover");
        });

        zone.addEventListener("dragleave", function() {
            depth = Math.max(0, depth - 1);
            if (!depth) zone.classList.remove("is-dragover");
        });

        zone.addEventListener("dragend", reset);

        zone.addEventListener("drop", function(e) {
            if (!hasFiles(e)) return;
            e.preventDefault();
            reset();
            if (input.disabled) return;
            var files = e.dataTransfer.files;
            if (files && files.length > 1) {
                show("Drop one file at a time. Your current selection was kept.");
                return;
            }
            var fallback = "Couldn't add the dropped file. Choose it with the file field instead; your current selection was kept.";
            if (!files || files.length !== 1 || typeof Event !== "function") {
                show(fallback);
                return;
            }
            var previous = input.files;
            var assigned = false;
            try {
                input.files = files;
                assigned = !!input.files && input.files.length === 1 && input.files[0].name === files[0].name && input.files[0].size === files[0].size;
            } catch (error) {
                assigned = false;
            }
            if (!assigned) {
                try {
                    if (previous && input.files !== previous) input.files = previous;
                } catch (error) {
                    assigned = false;
                }
                show(fallback);
                return;
            }
            show("");
            input.dispatchEvent(new Event("change", { bubbles: true }));
        });

        input.addEventListener("change", function() {
            show("");
        });
    });
}

$(initFileDrops);

function initValidationSummaries() {
    Array.prototype.forEach.call(document.querySelectorAll("form[data-validation-summary]"), function(form) {
        if (form.dataset.validationSummaryBound) return;
        form.dataset.validationSummaryBound = "1";

        var summary = document.createElement("div");
        summary.className = "form-notice form-validation-summary";
        summary.setAttribute("role", "alert");
        summary.tabIndex = -1;
        summary.hidden = true;
        var anchor = form.id === "login" ? form.querySelector(":scope > .login-field") : null;
        form.insertBefore(summary, anchor || form.firstElementChild);

        var owned = [];
        var attempted = false;
        var queued = false;
        var signature = "";

        var fieldLabel = function(control) {
            var label = "";
            if (control.labels && control.labels.length) {
                var copy = control.labels[0].cloneNode(true);
                Array.prototype.forEach.call(copy.querySelectorAll(".form-required-marker, .form-optional-marker"), function(marker) {
                    marker.remove();
                });
                label = copy.textContent;
            }
            return String(label).replace(/\s+/g, " ").trim() || control.name || control.id || "Field";
        };

        var release = function(entry) {
            if (entry.previous === null) entry.control.removeAttribute("aria-invalid");
            else entry.control.setAttribute("aria-invalid", entry.previous);
        };

        var mark = function(controls) {
            owned = owned.filter(function(entry) {
                if (controls.indexOf(entry.control) >= 0) return true;
                release(entry);
                return false;
            });
            controls.forEach(function(control) {
                var known = owned.some(function(entry) { return entry.control === control; });
                if (!known && control.getAttribute("aria-invalid") !== "true") {
                    owned.push({ control: control, previous: control.getAttribute("aria-invalid") });
                }
                if (owned.some(function(entry) { return entry.control === control; })) control.setAttribute("aria-invalid", "true");
            });
        };

        var render = function() {
            var controls = Array.prototype.filter.call(form.elements, function(control) {
                return control.willValidate && !control.validity.valid;
            });
            mark(controls);
            var items = controls.map(function(control) {
                return { control: control, label: fieldLabel(control), message: String(control.validationMessage || "") };
            });
            var next = items.map(function(item) {
                return Array.prototype.indexOf.call(form.elements, item.control) + "\u0000" + item.label + "\u0000" + item.message;
            }).join("\u0001");
            if (next === signature) return items.length;
            signature = next;
            summary.textContent = "";
            if (!items.length) {
                summary.hidden = true;
                return 0;
            }
            var heading = document.createElement("h2");
            heading.className = "form-validation-heading";
            heading.textContent = items.length === 1 ? "1 field needs attention" : items.length + " fields need attention";
            var list = document.createElement("ul");
            list.className = "form-validation-list";
            items.forEach(function(item) {
                var entry = document.createElement("li");
                var link = document.createElement("button");
                link.type = "button";
                link.className = "form-validation-link";
                link.textContent = item.message ? item.label + ": " + item.message : item.label;
                link.addEventListener("click", function() {
                    if (!item.control.isConnected) return;
                    item.control.scrollIntoView({ block: "center" });
                    item.control.focus({ preventScroll: true });
                });
                entry.appendChild(link);
                list.appendChild(entry);
            });
            summary.appendChild(heading);
            summary.appendChild(list);
            summary.hidden = false;
            return items.length;
        };

        var clear = function() {
            attempted = false;
            owned.forEach(release);
            owned = [];
            signature = "";
            summary.textContent = "";
            summary.hidden = true;
        };

        form.addEventListener("invalid", function(e) {
            e.preventDefault();
            attempted = true;
            if (queued) return;
            queued = true;
            setTimeout(function() {
                queued = false;
                if (render()) summary.focus();
            }, 0);
        }, true);

        var update = function() {
            if (attempted) render();
        };
        form.addEventListener("input", update);
        form.addEventListener("change", update);
        form.addEventListener("submit", clear);
    });
}

$(initValidationSummaries);

function initSettingsPasswordClear() {
    var toggle = document.getElementById("clear_password");
    var password = document.getElementById("smtp_password");
    var help = document.getElementById("password_help");
    if (!toggle || !password || !help || toggle.type !== "checkbox" || toggle.dataset.passwordClearBound) return;
    toggle.dataset.passwordClearBound = "1";
    var disabled = password.disabled;
    var text = help.textContent;

    var sync = function() {
        if (toggle.checked) {
            password.value = "";
            password.disabled = true;
            help.textContent = "The saved password will be removed when you save settings.";
        } else {
            password.disabled = disabled;
            help.textContent = text;
        }
    };

    toggle.addEventListener("change", sync);
    if (toggle.form) {
        toggle.form.addEventListener("reset", function() {
            setTimeout(sync, 0);
        });
    }
    sync();
}

$(initSettingsPasswordClear);

function initCrashRiskPreview() {
    var form = document.getElementById("editCrash");
    var input = document.getElementById("bugID");
    var preview = document.getElementById("bugIDRisk");
    if (!form || !input || !preview || !form.contains(input) || !form.contains(preview) || preview.dataset.riskPreviewBound) return;
    preview.dataset.riskPreviewBound = "1";
    var labels = { high: "High", medium: "Medium", low: "Low", none: "None" };

    input.addEventListener("input", function() {
        var match = /\w{2,3}(R|W|E)\W?/.exec(String(input.value));
        var risk = match ? { R: "low", W: "medium", E: "high" }[match[1]] : "none";
        preview.classList.remove("is-high", "is-medium", "is-low", "is-none", "is-na");
        preview.classList.add("is-" + risk);
        preview.textContent = "Risk preview: " + labels[risk] + " (unsaved)";
    });
}

$(initCrashRiskPreview);

var plotDataLimit = 10*1024*1024;
var plotDataMaxRows = 50000;
var plotDataColumns = [0, 1, 2, 3, 4, 5, 7, 8, 9, 10];
var plotDataRequests = {};
var plotSvgNamespace = "http://www.w3.org/2000/svg";
var plotCharts = {
    "exec_speed.png": { height: 230, integer: false, series: [["Exec speed", 10, 1]] },
    "high_freq.png": { height: 300, integer: true, series: [["Total paths", 3, 2], ["Current path", 2, 3], ["Pending paths", 4, 1], ["Pending favorites", 5, 4], ["Cycles done", 1, 5]] },
    "low_freq.png": { height: 230, integer: true, series: [["Unique crashes", 7, 4], ["Unique hangs", 8, 5], ["Levels", 9, 1]] }
};

function plotDataText(response) {
    var length = Number(response.headers.get("Content-Length"));
    if (length > plotDataLimit) throw new Error("Plot data is too large.");
    if (!response.body || typeof response.body.getReader !== "function" || typeof TextDecoder !== "function") {
        return response.text().then(function(text) {
            if (text.length > plotDataLimit) throw new Error("Plot data is too large.");
            return text;
        });
    }
    var reader = response.body.getReader();
    var decoder = new TextDecoder("utf-8", { fatal: true });
    var size = 0;
    var parts = [];
    var read = function() {
        return reader.read().then(function(result) {
            if (result.done) {
                parts.push(decoder.decode());
                return parts.join("");
            }
            size += result.value.byteLength;
            if (size > plotDataLimit) {
                reader.cancel();
                throw new Error("Plot data is too large.");
            }
            parts.push(decoder.decode(result.value, { stream: true }));
            return read();
        });
    };
    return read();
}

function plotDataParse(text) {
    if (/^\s*</.test(text)) throw new Error("Plot data is not CSV.");
    var lines = text.split(/\r\n|\r|\n/);
    var rows = [];
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i].trim();
        if (!line || line.charAt(0) === "#") continue;
        var cells = line.split(",");
        if (cells.length < 11) throw new Error("Plot data has too few columns.");
        var row = [];
        for (var j = 0; j < plotDataColumns.length; j++) {
            var column = plotDataColumns[j];
            var cell = cells[column].trim();
            if (!/^\d+(?:\.\d+)?$/.test(cell)) throw new Error("Plot data has an invalid value.");
            var value = Number(cell);
            if (!isFinite(value) || value < 0) throw new Error("Plot data has an invalid value.");
            row[column] = value;
        }
        if (rows.length && row[0] < rows[rows.length - 1][0]) throw new Error("Plot data time is out of order.");
        if (rows.length >= plotDataMaxRows) throw new Error("Plot data has too many rows.");
        rows.push(row);
    }
    if (!rows.length) throw new Error("Plot data is empty.");
    return rows;
}

function plotDataLoad(url) {
    if (!Object.prototype.hasOwnProperty.call(plotDataRequests, url)) {
        plotDataRequests[url] = fetch(url, { method: "GET", cache: "no-store", credentials: "same-origin" }).then(function(response) {
            var type = response.headers.get("Content-Type") || "";
            if (!response.ok || response.redirected || /html/i.test(type)) throw new Error("Plot data is unavailable.");
            return plotDataText(response);
        }).then(plotDataParse);
    }
    return plotDataRequests[url];
}

function plotSvg(parent, name, attributes) {
    var element = document.createElementNS(plotSvgNamespace, name);
    Object.keys(attributes || {}).forEach(function(key) {
        element.setAttribute(key, String(attributes[key]));
    });
    if (parent) parent.appendChild(element);
    return element;
}

function plotStep(raw, integer) {
    var power = Math.pow(10, Math.floor(Math.log(raw) / Math.LN10));
    var factors = integer ? [1, 2, 5, 10] : [1, 2, 2.5, 5, 10];
    for (var i = 0; i < factors.length; i++) {
        var step = factors[i] * power;
        if (step >= raw * (1 - 1e-9)) return integer ? Math.max(1, step) : step;
    }
    return 10 * power;
}

function plotTimeStep(span) {
    var target = span / 5;
    var steps = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 10800, 21600, 43200, 86400];
    for (var i = 0; i < steps.length; i++) {
        if (steps[i] >= target) return steps[i];
    }
    return 86400 * plotStep(target / 86400, true);
}

function plotNumber(value) {
    var abs = Math.abs(value);
    var units = [[1e9, "G"], [1e6, "M"], [1e3, "k"]];
    for (var i = 0; i < units.length; i++) {
        if (abs >= units[i][0]) return String(Number((value / units[i][0]).toFixed(1))) + units[i][1];
    }
    return String(Number(value.toFixed(abs < 10 ? 2 : 1)));
}

function plotValue(value) {
    return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

function plotDuration(seconds) {
    seconds = Math.round(seconds);
    if (seconds < 60) return seconds + "s";
    var days = Math.floor(seconds / 86400);
    var hours = Math.floor(seconds % 86400 / 3600);
    var minutes = Math.floor(seconds % 3600 / 60);
    if (days) return days + "d" + (hours ? " " + hours + "h" : "");
    if (hours) return hours + "h" + (minutes ? " " + minutes + "m" : "");
    return minutes + "m" + (seconds % 60 ? " " + seconds % 60 + "s" : "");
}

function plotSample(rows, column, x) {
    var picked = [];
    var bucket = null;
    var flush = function() {
        if (!bucket) return;
        [bucket.first, bucket.min, bucket.max, bucket.last].sort(function(a, b) {
            return a - b;
        }).forEach(function(i) {
            if (picked[picked.length - 1] !== i) picked.push(i);
        });
    };
    rows.forEach(function(row, i) {
        var px = Math.round(x(row[0]));
        if (!bucket || bucket.px !== px) {
            flush();
            bucket = { px: px, first: i, min: i, max: i, last: i };
            return;
        }
        if (row[column] < rows[bucket.min][column]) bucket.min = i;
        if (row[column] > rows[bucket.max][column]) bucket.max = i;
        bucket.last = i;
    });
    flush();
    return picked;
}

function plotChart(definition, rows, id, title) {
    var width = 1000;
    var height = definition.height;
    var left = 64;
    var right = 20;
    var top = 12;
    var bottom = 58;
    var plotWidth = width - left - right;
    var plotHeight = height - top - bottom;
    var first = rows[0][0];
    var last = rows[rows.length - 1][0];
    var span = last - first;
    var stats = definition.series.map(function(series) {
        var column = series[1];
        var max = 0;
        rows.forEach(function(row) {
            if (row[column] > max) max = row[column];
        });
        return { name: series[0], column: column, color: series[2], first: rows[0][column], max: max, last: rows[rows.length - 1][column] };
    });
    var max = Math.max.apply(null, stats.map(function(series) { return series.max; }));
    var step = plotStep((max || 1) / 4, definition.integer);
    var yMax = Math.max(step, Math.ceil(max / step) * step);
    if (!isFinite(step) || step <= 0 || !isFinite(yMax) || yMax <= 0) throw new Error("Plot data has an invalid scale.");
    var x = function(time) {
        return left + (span > 0 ? (time - first) / span * plotWidth : plotWidth / 2);
    };
    var y = function(value) {
        return top + plotHeight - value / yMax * plotHeight;
    };
    var format = function(value) {
        return value.toFixed(1);
    };

    var svg = plotSvg(null, "svg", {
        viewBox: "0 0 " + width + " " + height,
        class: "plot-chart-svg",
        role: "img",
        focusable: "false",
        "aria-labelledby": id + "-title",
        "aria-describedby": id + "-desc"
    });
    plotSvg(svg, "title", { id: id + "-title" }).textContent = title;
    plotSvg(svg, "desc", { id: id + "-desc" }).textContent = title + ": " + stats.map(function(series) {
        return series.name.toLowerCase();
    }).join(", ") + " over relative time from " + plotDuration(first) + " to " + plotDuration(last) + ", " + rows.length + (rows.length === 1 ? " row" : " rows") + " of plot data. Last values: " + stats.map(function(series) {
        return series.name + " " + plotValue(series.last);
    }).join(", ") + ".";

    var grid = plotSvg(svg, "g", { class: "plot-chart-grid", "aria-hidden": "true" });
    var axes = plotSvg(svg, "g", { class: "plot-chart-axes", "aria-hidden": "true" });
    for (var i = 0; i * step <= yMax * (1 + 1e-9); i++) {
        var value = Number((i * step).toPrecision(12));
        var py = format(y(value));
        plotSvg(grid, "line", { x1: left, x2: width - right, y1: py, y2: py });
        plotSvg(axes, "text", { x: left - 8, y: py, dy: "0.35em", "text-anchor": "end", class: "plot-chart-tick" }).textContent = plotNumber(value);
    }

    var ticks = [];
    if (span > 0) {
        var timeStep = plotTimeStep(span);
        for (var time = Math.ceil(first / timeStep) * timeStep, count = 0; time <= last && count < 20; time += timeStep, count++) {
            ticks.push(time);
        }
    }
    if (!ticks.length) ticks = span > 0 ? [first, last] : [first];
    ticks.forEach(function(time) {
        var px = x(time);
        plotSvg(grid, "line", { x1: format(px), x2: format(px), y1: top, y2: top + plotHeight });
        plotSvg(axes, "text", {
            x: format(px),
            y: top + plotHeight + 20,
            "text-anchor": px < left + 24 ? "start" : px > width - right - 24 ? "end" : "middle",
            class: "plot-chart-tick"
        }).textContent = plotDuration(time);
    });
    plotSvg(axes, "text", { x: format(left + plotWidth / 2), y: height - 8, "text-anchor": "middle", class: "plot-chart-axis-label" }).textContent = "Relative time";
    plotSvg(axes, "line", { x1: left, x2: left, y1: top, y2: top + plotHeight, class: "plot-chart-axis" });
    plotSvg(axes, "line", { x1: left, x2: width - right, y1: top + plotHeight, y2: top + plotHeight, class: "plot-chart-axis" });

    var layer = plotSvg(svg, "g", { "aria-hidden": "true" });
    stats.forEach(function(entry, index) {
        var group = plotSvg(layer, "g", { class: "plot-series-" + entry.color });
        if (rows.length === 1) {
            plotSvg(group, "circle", { class: "plot-chart-point", cx: format(x(first)), cy: format(y(entry.last)), r: 4 });
            return;
        }
        var points = plotSample(rows, entry.column, x).map(function(i) {
            return format(x(rows[i][0])) + "," + format(y(rows[i][entry.column]));
        }).join("L");
        if (index === 0) {
            plotSvg(group, "path", { class: "plot-chart-area", d: "M" + format(x(first)) + "," + format(y(0)) + "L" + points + "L" + format(x(last)) + "," + format(y(0)) + "Z" });
        }
        plotSvg(group, "path", { class: "plot-chart-line", d: "M" + points, "vector-effect": "non-scaling-stroke" });
    });

    var container = document.createElement("div");
    container.className = "plot-chart";
    var hint = document.createElement("p");
    hint.className = "plot-chart-hint";
    hint.textContent = "Scroll horizontally to inspect the chart.";
    container.appendChild(hint);
    var viewport = document.createElement("div");
    viewport.className = "plot-chart-viewport";
    viewport.tabIndex = 0;
    viewport.setAttribute("role", "region");
    viewport.setAttribute("aria-label", title + " chart");
    viewport.appendChild(svg);
    container.appendChild(viewport);

    var legend = document.createElement("ul");
    legend.className = "plot-chart-legend";
    legend.setAttribute("aria-label", title + " legend");
    stats.forEach(function(entry) {
        var item = document.createElement("li");
        var swatch = document.createElement("span");
        swatch.className = "plot-chart-swatch plot-series-" + entry.color;
        swatch.setAttribute("aria-hidden", "true");
        item.appendChild(swatch);
        item.appendChild(document.createTextNode(entry.name));
        legend.appendChild(item);
    });
    container.appendChild(legend);

    var details = document.createElement("details");
    details.className = "plot-chart-details";
    var summary = document.createElement("summary");
    summary.textContent = "Values as a table";
    details.appendChild(summary);
    var wrapper = document.createElement("div");
    wrapper.className = "plot-chart-table";
    var table = document.createElement("table");
    var caption = document.createElement("caption");
    caption.className = "sr-only";
    caption.textContent = title + " values";
    table.appendChild(caption);
    var head = document.createElement("thead");
    var headRow = document.createElement("tr");
    ["Series", "First", "Max", "Last"].forEach(function(label) {
        var cell = document.createElement("th");
        cell.scope = "col";
        cell.textContent = label;
        headRow.appendChild(cell);
    });
    head.appendChild(headRow);
    table.appendChild(head);
    var body = document.createElement("tbody");
    stats.forEach(function(entry) {
        var row = document.createElement("tr");
        var name = document.createElement("th");
        name.scope = "row";
        name.textContent = entry.name;
        row.appendChild(name);
        [entry.first, entry.max, entry.last].forEach(function(value) {
            var cell = document.createElement("td");
            cell.textContent = plotValue(value);
            row.appendChild(cell);
        });
        body.appendChild(row);
    });
    table.appendChild(body);
    wrapper.appendChild(table);
    details.appendChild(wrapper);
    container.appendChild(details);
    return container;
}

function initPlotRefresh() {
    var plots = document.getElementById("plots");
    if (!plots || plots.dataset.plotRefreshBound) return;
    plots.dataset.plotRefreshBound = "1";

    var button = document.getElementById("plot-refresh");
    var status = document.getElementById("plot-refresh-status");
    var back = document.getElementById("plot-back-link");
    var busy = false;
    var outdated = " Displayed plots may be out of date.";

    if (back && /^\/job\/[^/]+\/plot$/.test(window.location.pathname)) {
        back.setAttribute("href", window.location.pathname.replace(/\/plot$/, "/view"));
        back.textContent = "Back to run";
    }

    var plotPath = /^\/job\/[^/]+\/plot$/.test(window.location.pathname) ? window.location.pathname : "";
    var plotFids = typeof URLSearchParams === "function" ? new URLSearchParams(window.location.search).getAll("fid") : [];
    var currentFID = plotFids.length === 1 && /^[1-9]\d*$/.test(plotFids[0]) ? Number(plotFids[0]) : 0;
    var toolbar = button && button.parentNode && button.parentNode.classList.contains("plot-toolbar") ? button.parentNode : document.querySelector(".plot-toolbar");
    if (plotPath && Number.isSafeInteger(currentFID) && currentFID > 0 && toolbar && !toolbar.querySelector(".plot-instance") && typeof fetch === "function" && typeof URL === "function") {
        var jobPath = plotPath.replace(/\/plot$/, "");
        fetch(jobPath + "/export", { method: "GET", cache: "no-store", credentials: "same-origin", headers: { Accept: "application/json" } }).then(function(response) {
            var type = response.headers.get("Content-Type") || "";
            if (!response.ok || response.redirected || !/^application\/json\b/i.test(type)) throw new Error("Job configuration is unavailable.");
            return response.json();
        }).then(function(data) {
            if (data === null || typeof data !== "object" || Array.isArray(data)) return;
            if (typeof data.cores !== "number" || !Number.isSafeInteger(data.cores) || data.cores < 1 || data.cores > 4096) return;
            if (typeof data.banner !== "string" || typeof data.name !== "string") return;
            if (!toolbar.isConnected || toolbar.querySelector(".plot-instance")) return;
            var banner = data.banner.replace(/\s+/g, " ").trim();
            var name = data.name.replace(/\s+/g, " ").trim();

            var current = document.querySelector(".page-breadcrumbs .page-breadcrumb-current");
            if (name && current && current.parentNode && !current.parentNode.querySelector(".page-breadcrumb-job")) {
                var crumb = document.createElement("li");
                var crumbLink = document.createElement("a");
                crumbLink.className = "page-breadcrumb-parent page-breadcrumb-job";
                crumbLink.setAttribute("href", jobPath + "/view");
                crumbLink.textContent = name;
                crumb.appendChild(crumbLink);
                current.parentNode.insertBefore(crumb, current);
            }

            var controlId = "plot-instance-select";
            for (var n = 2; document.getElementById(controlId); n++) {
                controlId = "plot-instance-select-" + n;
            }
            var group = document.createElement("div");
            group.className = "plot-instance";
            var label = document.createElement("label");
            label.className = "form-label";
            label.setAttribute("for", controlId);
            label.textContent = "Instance";
            var select = document.createElement("select");
            select.id = controlId;
            select.className = "form-select";
            var option = function(fid) {
                var item = new Option((banner || "Instance ") + fid, String(fid));
                if (fid === currentFID) {
                    item.selected = true;
                    item.defaultSelected = true;
                }
                select.appendChild(item);
            };
            for (var fid = 1; fid <= data.cores; fid++) option(fid);
            if (currentFID > data.cores) option(currentFID);
            var help = document.createElement("small");
            help.className = "form-text plot-instance-help";
            help.id = controlId + "-help";
            help.textContent = "If the selected instance has no plots yet, use Refresh plot data to fetch them.";
            select.setAttribute("aria-describedby", help.id);
            select.addEventListener("change", function() {
                var value = String(select.value);
                if (!/^[1-9]\d*$/.test(value) || !Number.isSafeInteger(Number(value)) || Number(value) === currentFID) return;
                var target = new URL(window.location.href);
                target.searchParams.set("fid", value);
                window.location.assign(target.pathname + target.search);
            });
            group.appendChild(label);
            group.appendChild(select);
            group.appendChild(help);
            toolbar.insertBefore(group, button && button.parentNode === toolbar ? button : null);
        }).catch(function() {});
    }

    Array.prototype.forEach.call(plots.querySelectorAll(".plot-figure img"), function(image, index) {
        var figure = image.closest(".plot-figure");
        var plotLabels = {
            "exec_speed.png": ["Exec speed", "Executions per second over time"],
            "high_freq.png": ["Paths and cycles", "Total, current and pending paths, pending favorites and completed cycles"],
            "low_freq.png": ["Crashes, hangs and levels", "Unique crashes, unique hangs and path depth"]
        };
        var plotName = "";
        var plotUrl = null;
        try {
            plotUrl = new URL(image.getAttribute("src") || "", window.location.href);
            plotName = plotUrl.pathname.split("/").pop();
        } catch (error) {
            plotUrl = null;
            plotName = "";
        }
        var plotLabel = Object.prototype.hasOwnProperty.call(plotLabels, plotName)
            ? plotLabels[plotName]
            : ["Fuzzing plot", "Generated fuzzing statistics over time"];
        var plotTitle = document.createElement("h2");
        plotTitle.className = "plot-figure-title";
        plotTitle.textContent = plotLabel[0];
        var plotDescription = document.createElement("p");
        plotDescription.className = "plot-figure-description";
        plotDescription.textContent = plotLabel[1];
        image.parentNode.insertBefore(plotTitle, image);
        image.parentNode.insertBefore(plotDescription, image);
        image.alt = plotLabel[0];
        var open = figure.querySelector("figcaption a");
        if (open) open.setAttribute("aria-label", "Open image: " + plotLabel[0]);
        var message = document.createElement("p");
        message.className = "plot-refresh-status is-error plot-image-error";
        message.setAttribute("role", "status");
        message.hidden = true;
        var caption = figure.querySelector("figcaption");
        figure.insertBefore(message, caption && caption.parentNode === figure ? caption : null);
        var rendered = false;
        var failed = function() {
            if (rendered) return;
            image.hidden = true;
            message.hidden = false;
            message.textContent = "This plot image couldn't be loaded. Use Open image to view it directly.";
        };
        var loaded = function() {
            if (rendered) return;
            message.hidden = true;
            message.textContent = "";
            image.hidden = false;
        };
        image.addEventListener("error", failed);
        image.addEventListener("load", loaded);
        if (image.complete) {
            if (image.naturalWidth > 0) loaded();
            else failed();
        }

        var chart = Object.prototype.hasOwnProperty.call(plotCharts, plotName) ? plotCharts[plotName] : null;
        if (!chart || !plotUrl || plotUrl.origin !== window.location.origin || plotUrl.pathname.indexOf("/plots/") !== 0) return;
        if (typeof fetch !== "function" || typeof Promise !== "function") return;
        plotDataLoad(plotUrl.origin + plotUrl.pathname.replace(/[^/]*$/, "plot_data")).then(function(rows) {
            var element = plotChart(chart, rows, "plot-chart-" + index, plotLabel[0]);
            figure.insertBefore(element, message);
            rendered = true;
            image.hidden = true;
            message.hidden = true;
            message.textContent = "";
        }).catch(function() {
            if (rendered) return;
            var note = document.createElement("p");
            note.className = "plot-chart-note";
            note.textContent = "Chart data is unavailable. Showing the generated image when available.";
            figure.insertBefore(note, caption && caption.parentNode === figure ? caption : null);
        });
    });

    var show = function(text, state) {
        if (!status) return;
        status.textContent = text;
        status.classList.toggle("is-error", state === "error");
        status.classList.toggle("is-info", state === "info");
        status.hidden = false;
    };

    var refresh = function(manual) {
        if (busy) return;
        busy = true;
        if (manual && status) {
            status.hidden = true;
            status.textContent = "";
        }
        if (button) {
            button.disabled = true;
            button.textContent = "Refreshing…";
            button.setAttribute("aria-busy", "true");
        }
        $.ajax({
            url: window.location.href,
            method: "POST"
        }).done(function(data) {
            var json = data !== null && typeof data === "object" && typeof data.alert === "string";
            if (json && data.context === "success") {
                show("New plot data is available. This page will reload automatically.", "");
                if (manual) location.reload();
            } else if (json && (data.context === "info" || data.context === "danger")) {
                show(data.alert + outdated, data.context === "info" ? "info" : "error");
            } else {
                show("The server returned an unexpected response while refreshing plot data." + outdated, "error");
            }
        }).fail(function(jqXHR) {
            var json = jqXHR && jqXHR.responseJSON;
            if (json && typeof json.alert === "string") {
                show(json.alert + outdated, "error");
            } else if (jqXHR && jqXHR.status === 401) {
                show("You are not logged in. Please log in and try again." + outdated, "error");
            } else {
                show("Unable to refresh plot data. Check your connection and try again." + outdated, "error");
            }
        }).always(function() {
            busy = false;
            if (button) {
                button.disabled = false;
                button.textContent = "Refresh plot data";
                button.removeAttribute("aria-busy");
            }
        });
    };

    if (button) {
        button.addEventListener("click", function() {
            refresh(true);
        });
    }
    setInterval(function() {
        refresh(false);
    }, 1000*30);
    setInterval(function() {
        location.reload();
    }, 1000*100);
}

$(function () {
    var cards = document.getElementById("cards");
    if (!cards) return;
    cards.addEventListener("job-list-status-change", function () {
        var search = document.getElementById("search");
        if (filterCardsConfig !== "all" || (search && search.value && String(search.value).trim())) filterCards();
        else updateCardListToolbar();
    });
});

$(function () {
    var dialog = document.getElementById("job-config-dialog");
    if (!dialog || typeof dialog.showModal !== "function") return;
    var title = document.getElementById("job-config-title");
    var body = dialog.querySelector(".job-config-body");
    var close = dialog.querySelector(".job-config-close");
    var agent = dialog.querySelector(".job-config-agent");
    var job = dialog.querySelector(".job-config-job");
    var edit = dialog.querySelector(".job-config-edit");
    var trigger = null;
    var source = null;
    var observer = null;

    var syncEdit = function() {
        if (!edit) return false;
        var href = source && source.isConnected ? source.getAttribute("href") : "";
        var available = !!href && !source.classList.contains("disabled") && !source.classList.contains("d-none") && !source.hasAttribute("disabled") && source.getAttribute("aria-disabled") !== "true";
        if (available) edit.setAttribute("href", href);
        else edit.removeAttribute("href");
        edit.hidden = !available;
        return available;
    };

    if (edit) {
        edit.addEventListener("click", function(e) {
            if (!syncEdit()) e.preventDefault();
        });
    }

    document.querySelectorAll(".card.job .job-config").forEach(function(button) {
        button.addEventListener("click", function() {
            var card = button.closest(".card.job");
            var template = card ? card.querySelector("template.job-config-template") : null;
            if (!template || dialog.open) return;
            title.textContent = template.getAttribute("data-name") || "";
            if (agent) agent.textContent = template.getAttribute("data-agent") || "—";
            if (job) job.textContent = "Job #" + (template.getAttribute("data-job-id") || "");
            body.textContent = "";
            body.appendChild(template.content.cloneNode(true));
            body.scrollTop = 0;
            source = card.querySelector(".card-actions a.action.edit");
            if (source && edit && typeof MutationObserver === "function") {
                observer = new MutationObserver(syncEdit);
                observer.observe(source, { attributes: true, attributeFilter: ["class", "href", "disabled", "aria-disabled"] });
            }
            syncEdit();
            trigger = button;
            dialog.showModal();
            close.focus();
        });
    });

    close.addEventListener("click", function() {
        dialog.close();
    });

    dialog.addEventListener("click", function(e) {
        if (e.target === dialog) dialog.close();
    });

    dialog.addEventListener("close", function() {
        var target = trigger;
        trigger = null;
        if (observer) observer.disconnect();
        observer = null;
        source = null;
        if (agent) agent.textContent = "";
        if (job) job.textContent = "";
        syncEdit();
        if (target && target.isConnected && target.getClientRects().length) target.focus();
    });
});
