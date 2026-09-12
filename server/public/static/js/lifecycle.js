$(function () {
    $(".card.job[data-guid]").each(function () {
        var card = $(this), guid = card.attr("data-guid"), busy = false, checking = false;
        var startUnconfirmed = true;
        var progress = card.find(".job-progress"), instances = card.find(".instance-progress");
        var command = $("<div class='job-command-status small p-2' role='status'>").insertAfter(progress);
        var lastSnapshot = null, pendingStop = false, epoch = 0, commandUnconfirmed = false;
        function notify(message, context) {
            commandUnconfirmed = context === "danger";
            command.text(message);
            // Explicit action results use the existing page alert. Background
            // polling details remain inside the expandable job body.
            $("#alert").removeClass("alert-empty alert-danger alert-info alert-success")
                .addClass("alert-" + context).find(".message").text(message);
        }
        function active(s) { return ["queued", "starting", "bootstrapping", "running", "stopping", "stop_failed"].indexOf(s) >= 0; }
        function controls(anyActive, anyRunning) {
            var blocked = busy || pendingStop;
            card.find(".job-start[data-fid='0']").toggleClass("d-none", anyActive).removeClass("disabled").prop("disabled", blocked || startUnconfirmed);
            card.find(".job-stop").toggleClass("d-none", !anyActive && !pendingStop).prop("disabled", busy);
            card.find(".job-check,.view,.alarm,.collect").toggleClass("d-none", !anyRunning);
            // Configuration actions are available while no instance is active.
            card.find(".edit,.delete,.download").toggleClass("d-none", anyActive).toggleClass("disabled", blocked).prop("disabled", blocked);
        }
        function render(snapshot, stale) {
            startUnconfirmed = !!stale;
            if (snapshot && snapshot.instances && snapshot.instances.length) lastSnapshot = snapshot;
            if (!lastSnapshot) {
                // Keep configuration actions reachable if status polling fails.
                // Their server handlers validate the operation when requested.
                controls(false, false);
                return;
            }
            var anyActive = lastSnapshot.instances.some(function (i) { return active(i.status); });
            var anyRunning = lastSnapshot.instances.some(function (i) { return i.status === "running"; });
            instances.empty();
            lastSnapshot.instances.forEach(function (i) {
                var row = $("<div>").text("#" + i.fid + ": " + i.status + (i.error ? " - " + i.error : "") + (stale ? " (unconfirmed)" : ""));
                if (anyActive && !active(i.status)) {
                    $("<button class='btn btn-sm btn-outline-secondary job-start'>").text("Start").attr("data-fid", i.fid)
                        .prop("disabled", busy || stale || pendingStop).appendTo(row);
                }
                row.appendTo(instances);
            });
            controls(anyActive, anyRunning);
            if (pendingStop && !stale && !anyActive) { pendingStop = false; commandUnconfirmed = false; command.text(""); render(lastSnapshot, false); }
        }
        function check() {
            if (checking || busy) return;
            checking = true;
            var observedEpoch = epoch;
            $.ajax({url:"/job/" + guid + "/check", method:"POST", global:false, timeout:25000})
                .done(function (data) {
                    if (observedEpoch !== epoch) return;
                    render(data.snapshot, false);
                    progress.attr("title", "").text(data.alert || "Updated");
                    if (!commandUnconfirmed && !pendingStop) command.text("");
                })
                .fail(function (xhr) {
                    if (observedEpoch !== epoch) return;
                    var data = xhr.responseJSON || {};
                    render(data.snapshot, true);
                    var hasObservation = lastSnapshot && lastSnapshot.instances && lastSnapshot.instances.length;
                    var missing = data.error_kind === "job_missing";
                    var message = missing
                        ? "Job not registered on agent. Check the agent version."
                        : "Cannot refresh status. Check the agent connection.";
                    if (data.error_kind === "run_unrecognized") message = "Previous run or start request is unconfirmed. Start is unavailable.";
                    if (hasObservation) message += " Showing the last known state; current state is unconfirmed.";
                    progress.attr("title", data.error || "Request failed").text(message);
                }).always(function () { checking = false; if (observedEpoch !== epoch) check(); });
        }
        card.on("click", ".job-check", check);
        card.on("click", ".job-start,.job-stop", function () {
            if (busy || $(this).prop("disabled")) return;
            var start = $(this).hasClass("job-start"), fid = $(this).attr("data-fid") || "0";
            busy = true; epoch++; if (!start) pendingStop = true;
            card.find(".download").addClass("d-none");
            card.find(".edit,.delete").addClass("disabled");
            card.find(".job-start,.job-stop").prop("disabled", true);
            commandUnconfirmed = false;
            command.text(start ? "Submitting start..." : "Requesting stop...");
            $.ajax({url:"/job/"+guid+"/"+(start ? "start?fid="+fid : "stop"), method:"POST", global:false, timeout:35000})
                .done(function (data) {
                    notify(start ? "Start accepted; tracking instance progress." : "Stop accepted; waiting for termination.", "info");
                    render(data.snapshot, false);
                })
                .fail(function (xhr) { notify((start ? "Start" : "Stop") + " unconfirmed: " + ((xhr.responseJSON || {}).error || "Request failed"), "danger"); })
                .always(function () { busy = false; card.find(".job-stop").prop("disabled", false); check(); });
        });
        render(null, true); check(); setInterval(check, 3000);
    });
});
