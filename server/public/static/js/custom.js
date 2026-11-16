import { Tooltip } from './bootstrap.bundle.min.js';

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
        if (!$(context).is(".play")) {
            $(context).siblings().toggleClass("disabled");
        }
        else if ($(context).is(".last")) {
            $(context).siblings(".disabled").not(".play").toggleClass("disabled");
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

    if ($("#plots").length) {
        setInterval(function() {
            $.post(window.location.href, function(data) {});
        }, 1000*30);
        setInterval(function() {
            location.reload();
        }, 1000*100);
    }

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

var filterCards = function() {
    $('#cards').find('.col').removeClass('d-none');
    var filter = $("#search").val();
    if (filter) {
        $('#cards').find('.card .card-body:not(:contains("'+filter+'"))').parent().parent().parent().addClass('d-none');
        if ($.fn.masonry) $('#cards').masonry();
    }
}

$(window).on('load', function() {
    filterCards();
})

$('#search').on('keyup', function() {
    filterCards();
})
