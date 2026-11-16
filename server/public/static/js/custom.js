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

    $('.truncate').each(function() {
        var text = $(this).text();
        if (text.length > 120) {
            var truncated = text.trim().substring(0, 120).split(' ').slice(0, -1).join(' ');
            $(this).text(truncated.replace(/[!-\/:-@\[-`{-~]$/, '') + '...');
        }
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

    $("a.action").click(function(e) {
        var isCustom = $(this).attr("data-method");
        var isDisabled = $(this).is("a.disabled");

        if (isCustom || isDisabled) {
            e.preventDefault();
        }

        if (isCustom && !isDisabled) {
            $.ajax({
                url: $(this).attr('href'),
                method: $(this).attr("data-method"),
                context: $(this),
                dataType: $(this).attr("data-type") ? "binary" : "json",
                statusCode: {
                    401: function() {
                        setTimeout(function() {
                            window.location.href = "/user/login";
                        }, 1000*2)
                    }
                },
            }).done(function(data, textStatus, jqXHR ) {
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
                showAlert("danger", "You are not logged in. Please log in and try again.");
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
var filterCardsSummary = null;

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
    var group = document.createElement("div");
    group.className = definition.className;
    var label = document.createElement("label");
    label.className = "form-label";
    label.setAttribute("for", controlId);
    label.textContent = definition.label;
    var select = document.createElement("select");
    select.id = controlId;
    select.className = "form-select";
    select.appendChild(new Option(definition.allLabel, "all"));
    definition.items.forEach(function(item) {
        select.appendChild(new Option(item.label, item.id));
    });
    select.addEventListener("change", function() {
        definition.change(select.value);
    });
    group.appendChild(label);
    group.appendChild(select);
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

function updateJobListSummary() {
    var cards = document.getElementById("cards");
    if (!filterCardsSummary || !cards) return;
    var counts = { total: 0, running: 0, starting: 0, stopped: 0, unknown: 0 };
    Array.prototype.forEach.call(cards.children, function(child) {
        if (!child.classList.contains("col") || !child.querySelector(".card.job")) return;
        counts.total++;
        counts[cardListConfig(child)]++;
    });
    Object.keys(counts).forEach(function(key) {
        var value = String(counts[key]);
        if (filterCardsSummary[key].textContent !== value) filterCardsSummary[key].textContent = value;
    });
}

function updateCardListToolbar() {
    var cards = document.getElementById("cards");
    if (!filterCardsToolbar || !cards) return;
    var columns = Array.prototype.filter.call(cards.children, function(child) {
        return child.classList.contains("col");
    });
    var shown = columns.filter(function(column) {
        return !column.classList.contains("d-none") && !column.hidden;
    }).length;
    filterCardsToolbar.count.textContent = "Showing " + shown + " of " + columns.length + " on this page";
    filterCardsToolbar.buttons.forEach(function(button) {
        button.setAttribute("aria-pressed", String(button.filterValue === filterCardsConfig));
    });
    filterCardsToolbar.clear.hidden = !cardListFiltered();
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
        filterCardsEmpty = document.createElement("div");
        filterCardsEmpty.className = "empty-state card-list-empty";
        filterCardsEmpty.hidden = true;
        var heading = document.createElement("h2");
        heading.textContent = "No matching " + noun;
        var text = document.createElement("p");
        text.textContent = "No " + noun + " on this page match the current search and filters.";
        var actions = document.createElement("div");
        actions.className = "empty-state-actions";
        var clear = document.createElement("button");
        clear.type = "button";
        clear.className = "btn btn-outline theme-secondary";
        clear.textContent = "Clear filters";
        clear.addEventListener("click", clearCardFilters);
        actions.appendChild(clear);
        filterCardsEmpty.appendChild(heading);
        filterCardsEmpty.appendChild(text);
        filterCardsEmpty.appendChild(actions);
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

$('#search').on('input', function() {
    filterCards();
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

    if (panels.length) {
        var definitions = [];
        panels.forEach(function(panel) {
            panel.querySelectorAll('[data-toggle="tooltip"][title], [data-toggle="tooltip"][data-bs-original-title]').forEach(function(label) {
                var item = label.closest("li.list-group-item");
                if (!item || !panel.contains(item)) return;
                var text = (label.getAttribute("title") || label.getAttribute("data-bs-original-title") || "").replace(/\s+/g, " ").trim();
                if (!text || label.jobStatDefinition) return;
                var definition = document.createElement("span");
                definition.className = "job-stat-definition";
                definition.hidden = true;
                definition.textContent = text;
                label.jobStatDefinition = definition;
                item.appendChild(definition);
                definitions.push(definition);
            });
        });

        toolbar = document.createElement("div");
        toolbar.className = "job-stats-toolbar";
        var toolbarLabel = document.createElement("span");
        toolbarLabel.className = "job-stats-toolbar-label";
        toolbarLabel.textContent = "Fuzzer instances (" + panels.length + ")";
        var toggle = document.createElement("button");
        toggle.type = "button";
        toggle.className = "btn btn-outline theme-secondary job-definitions-toggle";
        toggle.setAttribute("aria-pressed", "false");
        toggle.textContent = "Show definitions";
        toggle.addEventListener("click", function() {
            var show = toggle.getAttribute("aria-pressed") !== "true";
            toggle.setAttribute("aria-pressed", String(show));
            toggle.textContent = show ? "Hide definitions" : "Show definitions";
            definitions.forEach(function(definition) {
                definition.hidden = !show;
            });
        });
        toolbar.appendChild(toolbarLabel);
        toolbar.appendChild(toggle);

        var accordion = stats.querySelector(".accordion");
        var anchor = accordion && accordion.parentNode ? accordion : panels[0];
        anchor.parentNode.insertBefore(toolbar, anchor);
    }

    if (panels.length < 2) return;

    var picker = document.createElement("div");
    picker.className = "job-instance-picker";
    picker.setAttribute("role", "group");
    picker.setAttribute("aria-label", "Fuzzer instances");

    var select = function(index) {
        panels.forEach(function(panel, i) {
            panel.hidden = i !== index;
            buttons[i].setAttribute("aria-pressed", String(i === index));
            buttons[i].tabIndex = i === index ? 0 : -1;
        });
    };

    var buttons = panels.map(function(panel, index) {
        if (!panel.id) panel.id = "job-instance-" + index;
        var name = panel.querySelector(".job-instance-name");
        var label = (name ? name.textContent : "").replace(/\s+/g, " ").trim();
        var button = document.createElement("button");
        button.type = "button";
        button.className = "job-instance-tile";
        button.textContent = String(index + 1);
        button.title = label;
        button.setAttribute("aria-label", "Instance " + (index + 1) + ": " + label);
        button.setAttribute("aria-controls", panel.id);
        button.addEventListener("click", function() {
            select(index);
        });
        button.addEventListener("keydown", function(e) {
            if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
            var target;
            if (e.key === "ArrowRight") target = (index + 1) % panels.length;
            else if (e.key === "ArrowLeft") target = (index + panels.length - 1) % panels.length;
            else if (e.key === "Home") target = 0;
            else if (e.key === "End") target = panels.length - 1;
            else return;
            e.preventDefault();
            select(target);
            buttons[target].focus();
        });
        picker.appendChild(button);
        return button;
    });

    if (toolbar && toolbar.parentNode) {
        toolbar.parentNode.insertBefore(picker, toolbar.nextSibling);
    } else {
        stats.insertBefore(picker, stats.firstChild);
    }
    select(0);
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
        var nav = document.createElement("nav");
        nav.className = "settings-navigation";
        nav.setAttribute("aria-label", "Form sections");

        var label = document.createElement("small");
        label.className = "settings-navigation-label";
        label.textContent = "Jump to section";
        nav.appendChild(label);

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
            link.textContent = String(legend.textContent).replace(/\s+/g, " ").trim() || "Section " + (sectionIndex + 1);
            nav.appendChild(link);
        });

        form.parentNode.insertBefore(nav, form);
        if (form.parentNode.classList.contains("settings-form-layout")) {
            form.parentNode.classList.add("has-section-navigation");
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

    var toolbar = document.createElement("div");
    toolbar.className = "card-list-toolbar";
    var count = document.createElement("span");
    count.className = "card-list-count";
    count.setAttribute("role", "status");
    toolbar.appendChild(count);

    var buttons = [];
    [
        [".card.agent", "Agent configuration", [["All", "all"], ["Enabled", "enabled"], ["Disabled", "disabled"]]],
        [".card.crash", "Crash verification", [["All", "all"], ["Verified", "verified"], ["Unverified", "unverified"], ["Unavailable", "unavailable"]]],
        [".card.job", "Job status", [["All", "all"], ["Running", "running"], ["Starting", "starting"], ["Stopped", "stopped"], ["Unknown", "unknown"]]]
    ].forEach(function(definition) {
        if (!columns.some(function(column) { return column.querySelector(definition[0]); })) return;
        var group = document.createElement("div");
        group.className = "card-list-filter";
        group.setAttribute("role", "group");
        group.setAttribute("aria-label", definition[1]);
        definition[2].forEach(function(option) {
            var button = document.createElement("button");
            button.type = "button";
            button.className = "btn btn-outline theme-secondary";
            button.textContent = option[0];
            button.filterValue = option[1];
            button.addEventListener("click", function() {
                filterCardsConfig = option[1];
                filterCards();
            });
            group.appendChild(button);
            buttons.push(button);
        });
        toolbar.appendChild(group);
    });

    var agents = [];
    columns.forEach(function(column) {
        var id = cardListAgent(column);
        if (!id || agents.some(function(agent) { return agent.id === id; })) return;
        var host = column.querySelector(".card.job .card-summary .host");
        var name = host ? String(host.textContent).replace(/\s+/g, " ").trim() : "";
        agents.push({ id: id, label: (name ? name + " " : "") + "(#" + id + ")" });
    });
    if (agents.length) {
        toolbar.appendChild(cardListSelect({
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
        crashJobs.push({ id: id, label: (name ? name + " " : "") + "(#" + id + ")" });
    });
    if (crashJobs.length) {
        toolbar.appendChild(cardListSelect({
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
        toolbar.appendChild(cardListSelect({
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
        toolbar.appendChild(cardListSelect({
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

    var clear = document.createElement("button");
    clear.type = "button";
    clear.className = "btn btn-outline theme-secondary card-list-clear";
    clear.textContent = "Clear filters";
    clear.hidden = true;
    clear.addEventListener("click", clearCardFilters);
    toolbar.appendChild(clear);

    toolbar.count = count;
    toolbar.buttons = buttons;
    toolbar.clear = clear;
    filterCardsToolbar = toolbar;
    var anchor = document.querySelector(".card-fold-tools");
    if (!anchor || anchor.parentNode !== cards.parentNode) anchor = cards;
    cards.parentNode.insertBefore(toolbar, anchor);

    if (columns.some(function(column) { return column.querySelector(".card.job"); })) {
        var summary = document.createElement("div");
        summary.className = "job-list-summary";
        summary.setAttribute("role", "group");
        summary.setAttribute("aria-label", "Job status summary");
        var scope = document.createElement("span");
        scope.className = "job-list-summary-scope";
        scope.textContent = "On this page";
        summary.appendChild(scope);
        filterCardsSummary = {};
        [["total", "Jobs"], ["running", "Running"], ["starting", "Starting"], ["stopped", "Stopped"], ["unknown", "Status unknown"]].forEach(function(entry) {
            var item = document.createElement("span");
            item.className = "job-list-summary-item is-" + entry[0];
            var dot = document.createElement("span");
            dot.className = "job-list-summary-dot";
            dot.setAttribute("aria-hidden", "true");
            var label = document.createElement("span");
            label.className = "job-list-summary-label";
            label.textContent = entry[1];
            var value = document.createElement("span");
            value.className = "job-list-summary-count";
            item.appendChild(dot);
            item.appendChild(label);
            item.appendChild(value);
            summary.appendChild(item);
            filterCardsSummary[entry[0]] = value;
        });
        cards.parentNode.insertBefore(summary, toolbar);
        updateJobListSummary();
    }
    updateCardListToolbar();
});

$(function () {
    var cards = document.getElementById("cards");
    if (!cards || !cards.parentNode || document.querySelector(".card-fold-tools")) return;
    if (!filterCardsToolbar || filterCardsToolbar.parentNode !== cards.parentNode) return;
    if (!cardFoldItems(cards).length) return;

    var tools = document.createElement("div");
    tools.className = "card-fold-tools";

    var button = document.createElement("button");
    button.type = "button";
    button.className = "btn btn-outline theme-secondary";
    button.textContent = "Expand all";
    button.setAttribute("aria-controls", "cards");
    button.setAttribute("aria-pressed", "false");
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
    tools.appendChild(button);

    ["shown.bs.collapse", "hidden.bs.collapse"].forEach(function(name) {
        cards.addEventListener(name, updateCardFoldTools);
    });

    filterCardsToolbar.appendChild(tools);
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
        updateJobListSummary();
        var search = document.getElementById("search");
        if (filterCardsConfig !== "all" || (search && search.value && String(search.value).trim())) filterCards();
    });
});

$(function () {
    var dialog = document.getElementById("job-config-dialog");
    if (!dialog || typeof dialog.showModal !== "function") return;
    var title = document.getElementById("job-config-title");
    var body = dialog.querySelector(".job-config-body");
    var close = dialog.querySelector(".job-config-close");
    var trigger = null;

    document.querySelectorAll(".card.job .job-config").forEach(function(button) {
        button.addEventListener("click", function() {
            var card = button.closest(".card.job");
            var template = card ? card.querySelector("template.job-config-template") : null;
            if (!template || dialog.open) return;
            title.textContent = template.getAttribute("data-name") || "";
            body.textContent = "";
            body.appendChild(template.content.cloneNode(true));
            body.scrollTop = 0;
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
        if (target && target.isConnected && target.getClientRects().length) target.focus();
    });
});
