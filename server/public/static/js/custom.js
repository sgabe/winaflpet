$(function () {

    $(document).ajaxStart(function(){
        $("#wait").attr("style", "display: flex !important");
    });

    $(document).ajaxComplete(function(){
        $("#wait").attr("style", "display: none !important");
    });

    $(function () {
        $('[data-toggle="tooltip"]').tooltip()
    })

    $('.truncate').succinct({
        size: 120
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
        $(".alert").removeClass("alert-empty alert-danger alert-info").addClass("alert-" + context);
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
                        }
                        else if ($(this).is("a.verify")) {
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

    $("#accordion .collapse").on('hide.bs.collapse', function (e) {
        $("#"+e.target.id).parent().animate({"padding-top": 0, "padding-bottom": 0});
    })

    $("#accordion .collapse").on('show.bs.collapse', function (e) {
        $("#"+e.target.id).parent().animate({"padding": "1.25rem"})
    })

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

});
