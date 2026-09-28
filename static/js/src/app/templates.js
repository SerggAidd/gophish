var templates = []
var icons = {
    "application/vnd.ms-excel": "fa-file-excel-o",
    "text/plain": "fa-file-text-o",
    "image/gif": "fa-file-image-o",
    "image/png": "fa-file-image-o",
    "application/pdf": "fa-file-pdf-o",
    "application/x-zip-compressed": "fa-file-archive-o",
    "application/x-gzip": "fa-file-archive-o",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": "fa-file-powerpoint-o",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "fa-file-word-o",
    "application/octet-stream": "fa-file-o",
    "application/x-msdownload": "fa-file-o"
}

var attachmentsTable = null
var aiAttachmentStateChanged = function () {}
var aiResetSharedState = function () {}

// Save attempts to POST to /templates/
function save(idx) {
    var template = {
        attachments: []
    }
    template.name = $("#name").val()
    template.subject = $("#subject").val()
    template.envelope_sender = $("#envelope-sender").val()
    template.html = CKEDITOR.instances["html_editor"].getData();
    // Fix the URL Scheme added by CKEditor (until we can remove it from the plugin)
    template.html = template.html.replace(/https?:\/\/{{\.URL}}/gi, "{{.URL}}")
    // If the "Add Tracker Image" checkbox is checked, add the tracker
    if ($("#use_tracker_checkbox").prop("checked")) {
        if (template.html.indexOf("{{.Tracker}}") == -1 &&
            template.html.indexOf("{{.TrackingUrl}}") == -1) {
            template.html = template.html.replace("</body>", "{{.Tracker}}</body>")
        }
    } else {
        // Otherwise, remove the tracker
        template.html = template.html.replace("{{.Tracker}}</body>", "</body>")
    }
    template.text = $("#text_editor").val()
    // Add the attachments
    $.each($("#attachmentsTable").DataTable().rows().data(), function (i, target) {
        template.attachments.push({
            name: unescapeHtml(target[1]),
            content: target[3],
            type: target[4],
        })
    })

    if (idx != -1) {
        template.id = templates[idx].id
        api.templateId.put(template)
            .success(function (data) {
                successFlash("Template edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit the template
        api.templates.post(template)
            .success(function (data) {
                successFlash("Template added successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    if ($.fn.DataTable.isDataTable("#attachmentsTable")) {
        $("#attachmentsTable").DataTable().clear().draw()
    }
    $("#name").val("")
    $("#subject").val("")
    $("#envelope-sender").val("")
    $("#text_editor").val("")
    $("#html_editor").val("")
    if (CKEDITOR.instances.html_editor) {
        CKEDITOR.instances.html_editor.setData("")
    }
    aiResetSharedState()
    $("#modal").modal('hide')
}

var deleteTemplate = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the template. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(templates[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.templateId.delete(templates[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if(result.value) {
            Swal.fire(
                'Template Deleted!',
                'This template has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function deleteTemplate(idx) {
    if (confirm("Delete " + templates[idx].name + "?")) {
        api.templateId.delete(templates[idx].id)
            .success(function (data) {
                successFlash(data.message)
                load()
            })
    }
}

function attach(files) {
    if (!$.fn.DataTable.isDataTable("#attachmentsTable")) {
        attachmentsTable = $("#attachmentsTable").DataTable({
            destroy: true,
            "order": [
                [1, "asc"]
            ],
            columnDefs: [{
                orderable: false,
                targets: "no-sort"
            }, {
                sClass: "datatable_hidden",
                targets: [3, 4]
            }]
        })
    } else {
        attachmentsTable = $("#attachmentsTable").DataTable()
    }

    $.each(files, function (i, file) {
        var reader = new FileReader()
        reader.onload = function () {
            var icon = icons[file.type] || "fa-file-o"
            attachmentsTable.row.add([
                '<i class="fa ' + icon + '"></i>',
                escapeHtml(file.name),
                '<span class="remove-row"><i class="fa fa-trash-o"></i></span>',
                reader.result.split(",")[1],
                file.type || "application/octet-stream"
            ]).draw()
            aiAttachmentStateChanged()
        }
        reader.onerror = function (e) {
            console.log(e)
        }
        reader.readAsDataURL(file)
    })
}

function edit(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    $("#attachmentUpload").unbind('click').click(function () {
        this.value = null
    })
    $("#html_editor").ckeditor()
    setupAutocomplete(CKEDITOR.instances["html_editor"])
    $("#attachmentsTable").show()
    attachmentsTable = $('#attachmentsTable').DataTable({
        destroy: true,
        "order": [
            [1, "asc"]
        ],
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }, {
            sClass: "datatable_hidden",
            targets: [3, 4]
        }]
    });
    var template = {
        attachments: []
    }
    if (idx != -1) {
        $("#templateModalLabel").text("Edit Template")
        template = templates[idx]
        $("#name").val(template.name)
        $("#subject").val(template.subject)
        $("#envelope-sender").val(template.envelope_sender)
        $("#html_editor").val(template.html)
        $("#text_editor").val(template.text)
        attachmentRows = []
        $.each(template.attachments, function (i, file) {
            var icon = icons[file.type] || "fa-file-o"
            // Add the record to the modal
            attachmentRows.push([
                '<i class="fa ' + icon + '"></i>',
                escapeHtml(file.name),
                '<span class="remove-row"><i class="fa fa-trash-o"></i></span>',
                file.content,
                file.type || "application/octet-stream"
            ])
        })
        attachmentsTable.rows.add(attachmentRows).draw()
        aiAttachmentStateChanged()
        if (template.html.indexOf("{{.Tracker}}") != -1) {
            $("#use_tracker_checkbox").prop("checked", true)
        } else {
            $("#use_tracker_checkbox").prop("checked", false)
        }

    } else {
        $("#templateModalLabel").text("New Template")
    }
    // Handle Deletion
    $("#attachmentsTable").unbind('click').on("click", "span>i.fa-trash-o", function () {
        attachmentsTable.row($(this).parents('tr'))
            .remove()
            .draw()
        aiAttachmentStateChanged()
    })
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })
    $("#attachmentUpload").unbind('click').click(function () {
        this.value = null
    })
    $("#html_editor").ckeditor()
    $("#attachmentsTable").show()
    attachmentsTable = $('#attachmentsTable').DataTable({
        destroy: true,
        "order": [
            [1, "asc"]
        ],
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }, {
            sClass: "datatable_hidden",
            targets: [3, 4]
        }]
    });
    var template = {
        attachments: []
    }
    template = templates[idx]
    $("#name").val("Copy of " + template.name)
    $("#subject").val(template.subject)
    $("#envelope-sender").val(template.envelope_sender)
    $("#html_editor").val(template.html)
    $("#text_editor").val(template.text)
    $.each(template.attachments, function (i, file) {
        var icon = icons[file.type] || "fa-file-o"
        // Add the record to the modal
        attachmentsTable.row.add([
            '<i class="fa ' + icon + '"></i>',
            escapeHtml(file.name),
            '<span class="remove-row"><i class="fa fa-trash-o"></i></span>',
            file.content,
            file.type || "application/octet-stream"
        ]).draw()
    })
    aiAttachmentStateChanged()
    // Handle Deletion
    $("#attachmentsTable").unbind('click').on("click", "span>i.fa-trash-o", function () {
        attachmentsTable.row($(this).parents('tr'))
            .remove()
            .draw()
        aiAttachmentStateChanged()
    })
    if (template.html.indexOf("{{.Tracker}}") != -1) {
        $("#use_tracker_checkbox").prop("checked", true)
    } else {
        $("#use_tracker_checkbox").prop("checked", false)
    }
}

function importEmail() {
    raw = $("#email_content").val()
    convert_links = $("#convert_links_checkbox").prop("checked")
    if (!raw) {
        modalError("No Content Specified!")
    } else {
        api.import_email({
                content: raw,
                convert_links: convert_links
            })
            .success(function (data) {
                $("#text_editor").val(data.text)
                $("#html_editor").val(data.html)
                $("#subject").val(data.subject)
                // If the HTML is provided, let's open that view in the editor
                if (data.html) {
                    CKEDITOR.instances["html_editor"].setMode('wysiwyg')
                    $('.nav-tabs a[href="#html"]').click()
                }
                $("#importEmailModal").modal("hide")
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function load() {
    $("#templateTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.templates.get()
        .success(function (ts) {
            templates = ts
            $("#loading").hide()
            if (templates.length > 0) {
                $("#templateTable").show()
                templateTable = $("#templateTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                templateTable.clear()
                templateRows = []
                $.each(templates, function (i, template) {
                    templateRows.push([
                        escapeHtml(template.name),
                        moment(template.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Template' onclick='edit(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Template' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Template' onclick='deleteTemplate(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                templateTable.rows.add(templateRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching templates")
        })
}

$(document).ready(function () {
    // Keep nested workflow modals above each other while using a single backdrop.
    // Bootstrap creates a new backdrop for every modal; hiding the extras prevents
    // the page from becoming progressively darker as the user moves through the flow.
    function normalizeModalStack() {
        var visibleModals = $('.modal:visible')
        var backdrops = $('.modal-backdrop')

        if (!visibleModals.length) {
            backdrops.remove()
            return
        }

        visibleModals.each(function (index) {
            $(this).css('z-index', 1050 + (index * 10))
        })

        backdrops.hide()
        backdrops.first().show().css('z-index', 1040)
        $(document.body).addClass('modal-open')
    }

    $('.modal').on('shown.bs.modal hidden.bs.modal', function () {
        setTimeout(normalizeModalStack, 0)
    });
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document)
            .off('focusin.bs.modal') // guard against infinite focus loop
            .on('focusin.bs.modal', $.proxy(function (e) {
                if (
                    this.$element[0] !== e.target && !this.$element.has(e.target).length
                    // CKEditor compatibility fix start.
                    &&
                    !$(e.target).closest('.cke_dialog, .cke').length
                    // CKEditor compatibility fix end.
                ) {
                    this.$element.trigger('focus');
                }
            }, this));
    };
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    $("#importEmailModal").on('hidden.bs.modal', function (event) {
        $("#email_content").val("")
    })
    CKEDITOR.on('dialogDefinition', function (ev) {
        // Take the dialog name and its definition from the event data.
        var dialogName = ev.data.name;
        var dialogDefinition = ev.data.definition;

        // Check if the definition is from the dialog window you are interested in (the "Link" dialog window).
        if (dialogName == 'link') {
            dialogDefinition.minWidth = 500
            dialogDefinition.minHeight = 100

            // Remove the linkType field
            var infoTab = dialogDefinition.getContents('info');
            infoTab.get('linkType').hidden = true;
        }
    });
    // AI template generation and difficulty evaluation
    (function () {
        var generationContext = null
        var evaluationContext = null
        var generatedTemplate = null
        var generatedEvaluation = null
        var evaluationFlowMode = null
        var pendingGenerationContext = null

        var currentEvaluationInput = null
        var currentEvaluation = null
        var evaluatedEmailChanged = false
        var revisionFlowMode = null
        var difficultyFlowMode = null

        var lastAppliedGenerationContext = null
        var lastAppliedEvaluationContext = null

        var sendingProfiles = []
        var sendingProfilesLoaded = false
        var pendingSimulatedSenderEmail = ""
        var attachmentsExplicitNone = false
        var attachmentReevaluationTimer = null
        var attachmentReevaluationInProgress = false
        var attachmentReevaluationPending = false
        var attachmentRevision = 0

        var difficultyLabels = {
            "very_difficult": "Very difficult",
            "moderately_difficult": "Moderately difficult",
            "moderately_to_least_difficult": "Moderately to Least difficult",
            "least_difficult": "Least difficult"
        }

        var cueCategoryLabels = {
            "few": "Few",
            "some": "Some",
            "many": "Many"
        }

        var premiseCategoryLabels = {
            "weak": "Weak",
            "medium": "Medium",
            "strong": "Strong"
        }

        function cloneObject(value) {
            if (!value) {
                return value
            }
            return JSON.parse(JSON.stringify(value))
        }

        function showAIState(state) {
            $("#aiGenerationForm, #aiGenerationLoading, #aiGenerationResult").hide()
            $("#aiFormActions, #aiResultActions").hide()

            if (state === "loading") {
                $("#aiModalCloseButton, #aiModalFooter").hide()
                $("#aiGenerationLoading").show()
                return
            }

            $("#aiModalCloseButton, #aiModalFooter").show()
            if (state === "form") {
                $("#aiGenerationForm, #aiFormActions").show()
            } else if (state === "result") {
                $("#aiGenerationResult, #aiResultActions").show()
            }
        }

        function showAIError(message) {
            $("#aiModalFlashes").html(
                '<div class="alert alert-danger">' + escapeHtml(message) + "</div>"
            )
        }

        function showEvaluationContextError(message) {
            $("#evaluationContextFlashes").html(
                '<div class="alert alert-danger">' + escapeHtml(message) + "</div>"
            )
        }

        function showEvaluationResultError(message) {
            $("#evaluationResultLoading").hide()
            $("#evaluationResultContent").hide()
            $("#evaluationResultCloseButton, #evaluationResultFooter").show()
            $("#evaluationResultFlashes").html(
                '<div class="alert alert-danger">' + escapeHtml(message) + "</div>"
            )
        }

        function showChangeDifficultyError(message) {
            $("#changeDifficultyFlashes").html(
                '<div class="alert alert-danger">' + escapeHtml(message) + "</div>"
            )
        }

        function showEmailRevisionError(message) {
            $("#emailRevisionFlashes").html(
                '<div class="alert alert-danger">' + escapeHtml(message) + "</div>"
            )
        }

        function responseMessage(response, fallback) {
            if (response && response.responseJSON && response.responseJSON.message) {
                return response.responseJSON.message
            }
            if (response && response.responseText) {
                return response.responseText
            }
            return fallback
        }

        function getCurrentEmail() {
            var html = ""
            if (CKEDITOR.instances.html_editor) {
                html = CKEDITOR.instances.html_editor.getData()
            } else {
                html = $("#html_editor").val()
            }

            html = (html || "").replace(/https?:\/\/{{\.URL}}/gi, "{{.URL}}")

            return {
                subject: $("#subject").val() || "",
                text: $("#text_editor").val() || "",
                html: html
            }
        }

        function applyEmailToTemplate(email) {
            $("#subject").val(email.subject || "")
            $("#text_editor").val(email.text || "")

            if (CKEDITOR.instances.html_editor) {
                CKEDITOR.instances.html_editor.setData(email.html || "")
            } else {
                $("#html_editor").val(email.html || "")
            }
        }

        function emailHasContent(email) {
            return $.trim(email.subject || "") !== "" ||
                $.trim(email.text || "") !== "" ||
                $.trim(email.html || "") !== ""
        }

        function normalizedEmail(email) {
            email = email || {}
            return {
                subject: email.subject || "",
                text: email.text || "",
                html: email.html || ""
            }
        }

        function emailsEqual(left, right) {
            return JSON.stringify(normalizedEmail(left)) === JSON.stringify(normalizedEmail(right))
        }

        function emailDiffersFromTemplate(email) {
            return !emailsEqual(email, getCurrentEmail())
        }

        function getGenerationRequest() {
            return {
                target_audience: $("#ai_target_audience").val() || "",
                recipient_role: $("#ai_recipient_role").val() || "",
                organization_context: $("#ai_organization_context").val() || "",
                sender_context: $("#ai_sender_context").val() || "",
                scenario: $("#ai_scenario").val() || "",
                custom_scenario: $("#ai_custom_scenario").val() || "",
                language: $("#ai_language").val() || "",
                target_difficulty: $("#ai_difficulty").val() || "",
                additional_instructions: $("#ai_additional_instructions").val() || ""
            }
        }

        function validateGenerationRequest(request) {
            if (!$.trim(request.target_audience)) {
                return "Target Audience is required."
            }

            if (request.scenario === "custom" && !$.trim(request.custom_scenario)) {
                return "Custom scenario description is required."
            }

            return ""
        }

        function getManualEvaluationGenerationContext() {
            return {
                target_audience: $("#evaluation_target_audience").val() || "",
                recipient_role: $("#evaluation_recipient_role").val() || "",
                organization_context: $("#evaluation_organization_context").val() || "",
                sender_context: $("#evaluation_sender_context").val() || "",
                scenario: $("#evaluation_manual_scenario").val() || "",
                custom_scenario: $("#evaluation_manual_custom_scenario").val() || "",
                language: $("#evaluation_manual_language").val() || "",
                target_difficulty: "",
                additional_instructions: ""
            }
        }

        function populateManualEvaluationContext(context) {
            if (!context) {
                return
            }

            $("#evaluation_target_audience").val(context.target_audience || "")
            $("#evaluation_recipient_role").val(context.recipient_role || "")
            $("#evaluation_organization_context").val(context.organization_context || "")
            $("#evaluation_sender_context").val(context.sender_context || "")
            $("#evaluation_manual_scenario").val(context.scenario || "")
            $("#evaluation_manual_custom_scenario").val(context.custom_scenario || "")
            $("#evaluation_manual_language").val(context.language || "")
            updateManualScenarioFields()
        }

        function getSharedAttachmentRows() {
            var files = []

            if (!$.fn.DataTable.isDataTable("#attachmentsTable")) {
                return files
            }

            var table = $("#attachmentsTable").DataTable()
            var indexes = table.rows().indexes().toArray()

            $.each(indexes, function (i, rowIndex) {
                var row = table.row(rowIndex).data()
                if (!row) {
                    return
                }
                files.push({
                    row_index: rowIndex,
                    name: unescapeHtml(row[1]),
                    type: row[4] || "application/octet-stream"
                })
            })

            return files
        }

        function getAttachmentMetadata() {
            var files = []
            $.each(getSharedAttachmentRows(), function (i, file) {
                files.push({
                    name: file.name,
                    type: file.type
                })
            })
            return files
        }

        function attachmentSummaryHtml(files) {
            var html = '<table class="table" style="margin-bottom:20px;">' +
                '<thead><tr>' +
                '<th class="col-md-1"></th>' +
                '<th class="col-md-10">Name</th>' +
                '<th class="col-md-1"></th>' +
                '</tr></thead><tbody>'

            if (!files.length) {
                html += '<tr><td></td><td class="text-muted">No files attached.</td><td></td></tr>'
            } else {
                $.each(files, function (i, file) {
                    var icon = icons[file.type] || "fa-file-o"
                    html += '<tr>' +
                        '<td><i class="fa ' + escapeHtml(icon) + '"></i></td>' +
                        '<td>' + escapeHtml(file.name) + '</td>' +
                        '<td><span class="remove-row ai-remove-attachment" ' +
                        'data-attachment-row="' + escapeHtml(String(file.row_index)) + '" ' +
                        'title="Remove attachment" aria-label="Remove attachment">' +
                        '<i class="fa fa-trash-o"></i></span></td>' +
                        '</tr>'
                })
            }

            html += '</tbody></table>'
            return html
        }

        function removeSharedAttachment(rowIndex) {
            if (!$.fn.DataTable.isDataTable("#attachmentsTable")) {
                return
            }

            var table = $("#attachmentsTable").DataTable()
            var row = table.row(rowIndex)
            if (!row.data()) {
                return
            }

            row.remove().draw()
            if (getAttachmentMetadata().length === 0) {
                attachmentsExplicitNone = true
            }
            aiAttachmentStateChanged()
        }

        function syncNoAttachmentCheckboxes() {
            var files = getAttachmentMetadata()
            var hasFiles = files.length > 0

            if (hasFiles) {
                attachmentsExplicitNone = false
            }

            $("#ai_no_attachments, #evaluation_no_attachments")
                .prop("checked", !hasFiles && attachmentsExplicitNone)
                .prop("disabled", hasFiles)
        }

        function refreshAttachmentSummaries() {
            var files = getSharedAttachmentRows()
            var summary = attachmentSummaryHtml(files)

            $("#aiAttachmentsSummary").html(summary)
            $("#evaluationAttachmentsSummary").html(summary)
            $("#aiResultAttachmentsSummary").html(summary)
            $("#evaluationResultAttachmentsSummary").html(summary)
            syncNoAttachmentCheckboxes()
        }

        aiAttachmentStateChanged = function () {
            attachmentRevision += 1
            if (getAttachmentMetadata().length > 0) {
                attachmentsExplicitNone = false
            }
            refreshAttachmentSummaries()
            scheduleAttachmentReevaluation()
        }

        function activeAttachmentResultMode() {
            if ($("#GenerateWithAIModal").hasClass("in") && $("#aiGenerationResult").is(":visible") &&
                generatedTemplate && generationContext && evaluationContext) {
                return "generate"
            }
            if ($("#EvaluationResultModal").hasClass("in") && $("#evaluationResultContent").is(":visible") &&
                currentEvaluationInput) {
                return "evaluate"
            }
            return null
        }

        function scheduleAttachmentReevaluation(forcedMode) {
            var mode = forcedMode || activeAttachmentResultMode()
            if (forcedMode && !(mode === "generate" ? $("#GenerateWithAIModal") :
                $("#EvaluationResultModal")).hasClass("in")) {
                return
            }
            if (!mode) {
                return
            }
            if (attachmentReevaluationInProgress) {
                attachmentReevaluationPending = true
                return
            }
            if (attachmentReevaluationTimer) {
                clearTimeout(attachmentReevaluationTimer)
            }
            attachmentReevaluationTimer = setTimeout(function () {
                attachmentReevaluationTimer = null
                reevaluateAfterAttachmentChange(mode)
            }, 350)
        }

        function reevaluateAfterAttachmentChange(mode) {
            var attachmentContext = buildAttachmentContext()
            var input
            var requestedRevision = attachmentRevision

            if (mode === "generate") {
                if (!generatedTemplate || !generationContext || !evaluationContext) {
                    return
                }
                evaluationContext.attachments = cloneObject(attachmentContext)
                input = {
                    email: normalizedEmail(generatedTemplate),
                    generation_context: cloneObject(generationContext),
                    evaluation_context: cloneObject(evaluationContext)
                }
                $("#aiModalFlashes").empty()
                $("#aiLoadingText").text("Attachments changed. Re-evaluating email...")
                showAIState("loading")
            } else {
                if (!currentEvaluationInput) {
                    return
                }
                currentEvaluationInput.evaluation_context = currentEvaluationInput.evaluation_context || {}
                currentEvaluationInput.evaluation_context.attachments = cloneObject(attachmentContext)
                input = cloneObject(currentEvaluationInput)
                showEvaluationResultLoading()
            }

            attachmentReevaluationInProgress = true
            query("/ai/templates/evaluate", "POST", input, true)
                .success(function (evaluation) {
                    if (requestedRevision !== attachmentRevision) {
                        attachmentReevaluationPending = true
                        return
                    }
                    if (mode === "generate") {
                        generatedEvaluation = evaluation
                        renderGenerationResult({
                            email: generatedTemplate,
                            evaluation: evaluation
                        })
                        $("#aiAdjustmentMeta").html(
                            '<div class="alert alert-info"><strong>Attachments changed. Assessment updated (a previous result may have been reused).</strong></div>'
                        )
                    } else {
                        currentEvaluationInput = input
                        currentEvaluation = evaluation
                        renderEvaluationResult(
                            evaluation,
                            currentEvaluationInput.email,
                            emailDiffersFromTemplate(currentEvaluationInput.email),
                            null
                        )
                        $("#evaluationResultMeta").html(
                            '<div class="alert alert-info"><strong>Attachments changed. Assessment updated (a previous result may have been reused).</strong></div>'
                        )
                    }
                })
                .error(function (response) {
                    if (requestedRevision !== attachmentRevision) {
                        attachmentReevaluationPending = true
                        return
                    }
                    var message = responseMessage(response, "Failed to re-evaluate email after attachment change.")
                    if (mode === "generate") {
                        showAIState("result")
                        showAIError(message)
                    } else {
                        showEvaluationResultError(message)
                    }
                })
                .always(function () {
                    attachmentReevaluationInProgress = false
                    if (attachmentReevaluationPending) {
                        attachmentReevaluationPending = false
                        scheduleAttachmentReevaluation(mode)
                    }
                })
        }

        function refreshResult(mode) {
            if (attachmentReevaluationInProgress) {
                return
            }
            var input
            if (mode === "generate") {
                if (!generatedTemplate || !generationContext || !evaluationContext) {
                    return
                }
                evaluationContext.attachments = cloneObject(buildAttachmentContext())
                input = {
                    email: normalizedEmail(generatedTemplate),
                    generation_context: cloneObject(generationContext),
                    evaluation_context: cloneObject(evaluationContext)
                }
                $("#aiModalFlashes").empty()
                $("#aiLoadingText").text("Requesting a fresh model assessment...")
                showAIState("loading")
            } else {
                if (!currentEvaluationInput) {
                    return
                }
                input = cloneObject(currentEvaluationInput)
                input.evaluation_context.attachments = cloneObject(buildAttachmentContext())
                showEvaluationResultLoading()
            }
            var requestedRevision = attachmentRevision
            input.evaluation_mode = "refresh"
            attachmentReevaluationInProgress = true
            query("/ai/templates/evaluate", "POST", input, true)
                .success(function (evaluation) {
                    if (requestedRevision !== attachmentRevision) {
                        attachmentReevaluationPending = true
                        return
                    }
                    if (mode === "generate") {
                        renderGenerationResult({email: generatedTemplate, evaluation: evaluation})
                        $("#aiAdjustmentMeta").html(
                            '<div class="alert alert-info"><strong>Fresh model assessment completed.</strong></div>'
                        )
                    } else {
                        delete input.evaluation_mode
                        currentEvaluationInput = input
                        renderEvaluationResult(evaluation, input.email,
                            emailDiffersFromTemplate(input.email), null)
                        $("#evaluationResultMeta").html(
                            '<div class="alert alert-info"><strong>Fresh model assessment completed.</strong></div>'
                        )
                    }
                })
                .error(function (response) {
                    if (requestedRevision !== attachmentRevision) {
                        attachmentReevaluationPending = true
                        return
                    }
                    var message = responseMessage(response, "Failed to re-evaluate email.")
                    if (mode === "generate") {
                        showAIState("result")
                        showAIError(message)
                    } else {
                        renderEvaluationResult(currentEvaluation, currentEvaluationInput.email,
                            emailDiffersFromTemplate(currentEvaluationInput.email), null)
                        $("#evaluationResultFlashes").html(
                            '<div class="alert alert-danger">' + escapeHtml(message) + '</div>'
                        )
                    }
                })
                .always(function () {
                    attachmentReevaluationInProgress = false
                    if (attachmentReevaluationPending) {
                        attachmentReevaluationPending = false
                        scheduleAttachmentReevaluation(mode)
                    }
                })
        }

        aiResetSharedState = function () {
            attachmentRevision += 1
            attachmentsExplicitNone = false
            if (attachmentReevaluationTimer) {
                clearTimeout(attachmentReevaluationTimer)
                attachmentReevaluationTimer = null
            }
            attachmentReevaluationInProgress = false
            attachmentReevaluationPending = false
            generationContext = null
            evaluationContext = null
            generatedTemplate = null
            generatedEvaluation = null
            evaluationFlowMode = null
            pendingGenerationContext = null
            currentEvaluationInput = null
            currentEvaluation = null
            evaluatedEmailChanged = false
            revisionFlowMode = null
            difficultyFlowMode = null
            lastAppliedGenerationContext = null
            lastAppliedEvaluationContext = null
            pendingSimulatedSenderEmail = ""

            $("#ai_target_audience, #ai_recipient_role, #ai_organization_context, " +
                "#ai_sender_context, #ai_custom_scenario, #ai_additional_instructions").val("")
            $("#ai_scenario").val("password_expiration")
            $("#ai_language").val("en")
            $("#ai_difficulty").val("very_difficult")
            $("#ai_custom_scenario_group").hide()

            $("#evaluation_target_audience, #evaluation_recipient_role, " +
                "#evaluation_organization_context, #evaluation_sender_context, " +
                "#evaluation_manual_custom_scenario, #evaluation_simulated_sender_name, " +
                "#evaluation_expected_sender_name, #evaluation_expected_sender_email, " +
                "#evaluation_situation_context, #evaluation_simulated_url, " +
                "#evaluation_expected_domain").val("")
            $("#evaluation_manual_scenario").val("")
            $("#evaluation_manual_language").val("")
            $("#evaluation_manual_custom_scenario_group").hide()
            $("#evaluation_prior_training").val("unknown")
            $("#evaluation_sending_profile").val("")
            $("#evaluation_link_usage").val("unknown")
            $("#evaluationLinkFields").hide()
            $("#evaluationSendingProfilePreview").hide().empty()

            $("#change_difficulty_target").val("very_difficult")
            $("#change_difficulty_feedback").val("")
            $("#change_difficulty_iterations").val("3")
            $("#email_revision_feedback").val("")

            refreshAttachmentSummaries()
        }

        function setExplicitNoAttachments(value) {
            if (getAttachmentMetadata().length > 0) {
                attachmentsExplicitNone = false
            } else {
                attachmentsExplicitNone = value
            }
            syncNoAttachmentCheckboxes()
        }

        function buildAttachmentContext() {
            var files = getAttachmentMetadata()

            if (files.length > 0) {
                return {
                    usage: "used",
                    files: files
                }
            }

            if (attachmentsExplicitNone) {
                return {
                    usage: "none"
                }
            }

            return {
                usage: "unknown"
            }
        }

        function parseMailbox(value) {
            var raw = $.trim(value || "")
            var match
            var displayName = ""
            var email = ""

            if (!raw) {
                return null
            }

            match = raw.match(/^\s*(.*?)\s*<([^<>]+)>\s*$/)
            if (match) {
                displayName = $.trim(match[1] || "").replace(/^["']|["']$/g, "")
                email = $.trim(match[2] || "")
            } else if (raw.indexOf("@") !== -1) {
                email = raw
            } else {
                displayName = raw
            }

            return {
                display_name: displayName,
                email: email
            }
        }

        function findSendingProfile(id) {
            var result = null
            $.each(sendingProfiles, function (i, profile) {
                if (String(profile.id) === String(id)) {
                    result = profile
                    return false
                }
            })
            return result
        }

        function selectedSimulatedSender() {
            var id = $("#evaluation_sending_profile").val()
            var displayName = $.trim($("#evaluation_simulated_sender_name").val() || "")
            var profile
            var parsed

            if (!id) {
                return null
            }

            profile = findSendingProfile(id)
            if (!profile) {
                return null
            }

            parsed = parseMailbox(profile.from_address)
            if (!parsed) {
                return null
            }

            return {
                display_name: displayName || parsed.display_name || "",
                email: parsed.email || ""
            }
        }

        function updateSendingProfilePreview() {
            var profile = findSendingProfile($("#evaluation_sending_profile").val())

            if (!profile) {
                $("#evaluationSendingProfilePreview").hide().empty()
                return
            }

            $("#evaluationSendingProfilePreview")
                .html(
                    "<strong>" + escapeHtml(profile.name || "Sending Profile") + "</strong><br>" +
                    "From: " + escapeHtml(profile.from_address || "Not configured")
                )
                .show()
        }

        function selectProfileForSender(sender) {
            pendingSimulatedSenderEmail = sender && sender.email ? String(sender.email).toLowerCase() : ""

            if (!sendingProfilesLoaded || !pendingSimulatedSenderEmail) {
                return
            }

            var matched = ""
            $.each(sendingProfiles, function (i, profile) {
                var parsed = parseMailbox(profile.from_address)
                if (parsed && String(parsed.email || "").toLowerCase() === pendingSimulatedSenderEmail) {
                    matched = String(profile.id)
                    return false
                }
            })

            $("#evaluation_sending_profile").val(matched)
            updateSendingProfilePreview()
        }

        function populateSendingProfiles(profiles) {
            var current = $("#evaluation_sending_profile").val()
            sendingProfiles = profiles || []
            sendingProfilesLoaded = true

            $("#evaluation_sending_profile")
                .empty()
                .append('<option value="">Unknown / not selected</option>')

            $.each(sendingProfiles, function (i, profile) {
                var label = profile.name || "Unnamed profile"
                if (profile.from_address) {
                    label += " — " + profile.from_address
                }

                $("#evaluation_sending_profile").append(
                    $("<option></option>")
                        .attr("value", profile.id)
                        .text(label)
                )
            })

            if (current) {
                $("#evaluation_sending_profile").val(current)
            }

            if (pendingSimulatedSenderEmail) {
                selectProfileForSender({ email: pendingSimulatedSenderEmail })
            }

            updateSendingProfilePreview()
        }

        function loadSendingProfiles() {
            query("/smtp/", "GET", {}, true)
                .success(function (profiles) {
                    populateSendingProfiles(profiles)
                })
                .error(function () {
                    $("#evaluation_sending_profile")
                        .empty()
                        .append('<option value="">Unable to load Sending Profiles</option>')
                    $("#evaluationContextFlashes").html(
                        '<div class="alert alert-warning">Unable to load GoPhish Sending Profiles. ' +
                        "The simulated sender can remain unknown.</div>"
                    )
                })
        }

        function buildEvaluationContext() {
            var expectedName = $.trim($("#evaluation_expected_sender_name").val() || "")
            var expectedEmail = $.trim($("#evaluation_expected_sender_email").val() || "")
            var expectedSender = null
            var linkUsage = $("#evaluation_link_usage").val() || "unknown"
            var link = {
                usage: linkUsage
            }

            if (expectedName || expectedEmail) {
                expectedSender = {
                    display_name: expectedName,
                    email: expectedEmail
                }
            }

            if (linkUsage === "used") {
                link.simulated_url = $.trim($("#evaluation_simulated_url").val() || "")
                link.expected_domain = $.trim($("#evaluation_expected_domain").val() || "")
            }

            return {
                prior_training_exposure: $("#evaluation_prior_training").val() || "unknown",
                simulated_sender: selectedSimulatedSender(),
                expected_sender: expectedSender,
                situation_context: $("#evaluation_situation_context").val() || "",
                link: link,
                attachments: buildAttachmentContext()
            }
        }

        function applyEvaluationContextToForm(context) {
            if (!context) {
                return
            }

            $("#evaluation_prior_training").val(context.prior_training_exposure || "unknown")
            $("#evaluation_simulated_sender_name").val(
                context.simulated_sender ? context.simulated_sender.display_name || "" : ""
            )
            $("#evaluation_expected_sender_name").val(
                context.expected_sender ? context.expected_sender.display_name || "" : ""
            )
            $("#evaluation_expected_sender_email").val(
                context.expected_sender ? context.expected_sender.email || "" : ""
            )
            $("#evaluation_situation_context").val(context.situation_context || "")

            var link = context.link || {}
            $("#evaluation_link_usage").val(link.usage || "unknown")
            $("#evaluation_simulated_url").val(link.simulated_url || "")
            $("#evaluation_expected_domain").val(link.expected_domain || "")
            updateLinkFields()

            if (context.attachments && context.attachments.usage === "none" &&
                getAttachmentMetadata().length === 0) {
                attachmentsExplicitNone = true
            }

            selectProfileForSender(context.simulated_sender)
            refreshAttachmentSummaries()
        }

        function updateLinkFields() {
            if ($("#evaluation_link_usage").val() === "used") {
                $("#evaluationLinkFields").show()
            } else {
                $("#evaluationLinkFields").hide()
                if ($("#evaluation_link_usage").val() === "none") {
                    $("#evaluation_simulated_url, #evaluation_expected_domain").val("")
                }
            }
        }

        function updateManualScenarioFields() {
            if ($("#evaluation_manual_scenario").val() === "custom") {
                $("#evaluation_manual_custom_scenario_group").show()
            } else {
                $("#evaluation_manual_custom_scenario_group").hide()
                if ($("#evaluation_manual_scenario").val() !== "custom") {
                    $("#evaluation_manual_custom_scenario").val("")
                }
            }
        }

        function openEvaluationContext(mode) {
            var email

            $("#evaluationContextFlashes").empty()
            evaluationFlowMode = mode
            loadSendingProfiles()
            refreshAttachmentSummaries()

            if (mode === "generate") {
                pendingGenerationContext = getGenerationRequest()

                var validationError = validateGenerationRequest(pendingGenerationContext)
                if (validationError) {
                    showAIError(validationError)
                    return
                }

                $("#manualEvaluationContextGroup").hide()
                $("#evaluationContextSubmitButton")
                    .html('<i class="fa fa-magic"></i> Generate and Evaluate')

                if (lastAppliedEvaluationContext) {
                    applyEvaluationContextToForm(lastAppliedEvaluationContext)
                }
            } else {
                email = getCurrentEmail()
                if (!emailHasContent(email)) {
                    modalError("Add a subject or email content before evaluation.")
                    return
                }

                $("#manualEvaluationContextGroup").show()
                $("#evaluationContextSubmitButton")
                    .html('<i class="fa fa-search"></i> Evaluate Email')

                if (lastAppliedGenerationContext) {
                    populateManualEvaluationContext(lastAppliedGenerationContext)
                }

                if (lastAppliedEvaluationContext) {
                    applyEvaluationContextToForm(lastAppliedEvaluationContext)
                }
            }

            $("#EvaluationContextModal").modal({
                backdrop: "static",
                keyboard: false,
                show: true
            })
        }

        function labelDifficulty(value) {
            return difficultyLabels[value] || humanizeIdentifier(value)
        }

        function labelCueCategory(value) {
            return cueCategoryLabels[value] || humanizeIdentifier(value)
        }

        function labelPremiseCategory(value) {
            return premiseCategoryLabels[value] || humanizeIdentifier(value)
        }

        function humanizeIdentifier(value) {
            var text = String(value || "").replace(/_/g, " ")
            if (!text) {
                return "-"
            }
            return text.charAt(0).toUpperCase() + text.slice(1)
        }

        function valueRange(min, max) {
            if (min === undefined || min === null) {
                return "-"
            }
            if (max === undefined || max === null || min === max) {
                return String(min)
            }
            return String(min) + "–" + String(max)
        }

        function categoryRange(evaluation, kind) {
            if (!evaluation) {
                return "-"
            }

            if (evaluation.category_resolved && evaluation.category) {
                return kind === "premise" ?
                    labelPremiseCategory(evaluation.category) :
                    labelCueCategory(evaluation.category)
            }

            if (evaluation.min_category || evaluation.max_category) {
                var minLabel = kind === "premise" ?
                    labelPremiseCategory(evaluation.min_category) :
                    labelCueCategory(evaluation.min_category)
                var maxLabel = kind === "premise" ?
                    labelPremiseCategory(evaluation.max_category) :
                    labelCueCategory(evaluation.max_category)

                if (minLabel === maxLabel) {
                    return minLabel
                }
                return minLabel + " – " + maxLabel
            }

            return "Unresolved"
        }

        function difficultyText(difficulty) {
            if (!difficulty) {
                return "-"
            }

            if (difficulty.resolved && difficulty.detection_difficulty) {
                return labelDifficulty(difficulty.detection_difficulty)
            }

            if (difficulty.possible_difficulties && difficulty.possible_difficulties.length) {
                return $.map(difficulty.possible_difficulties, function (value) {
                    return labelDifficulty(value)
                }).join(" / ")
            }

            return "Unresolved"
        }

        function evaluationSummaryHtml(evaluation) {
            if (!evaluation) {
                return ""
            }

            var cues = evaluation.cues || {}
            var premise = evaluation.premise_alignment || {}

            return '' +
                '<div class="row ai-evaluation-summary">' +
                    '<div class="col-sm-3"><div class="panel panel-default ai-evaluation-card"><div class="panel-body">' +
                        '<div class="text-muted">Difficulty</div><strong>' +
                        escapeHtml(difficultyText(evaluation.difficulty)) +
                        '</strong></div></div></div>' +
                    '<div class="col-sm-3"><div class="panel panel-default ai-evaluation-card"><div class="panel-body">' +
                        '<div class="text-muted">Cue Count</div><strong>' +
                        escapeHtml(valueRange(cues.min_count, cues.max_count)) +
                        '</strong><br><span class="text-muted">' +
                        escapeHtml(categoryRange(cues, "cue")) +
                        '</span></div></div></div>' +
                    '<div class="col-sm-3"><div class="panel panel-default ai-evaluation-card"><div class="panel-body">' +
                        '<div class="text-muted">Premise Alignment</div><strong>' +
                        escapeHtml(valueRange(premise.min_score, premise.max_score)) +
                        '</strong><br><span class="text-muted">' +
                        escapeHtml(categoryRange(premise, "premise")) +
                        '</span></div></div></div>' +
                    '<div class="col-sm-3"><div class="panel panel-default ai-evaluation-card"><div class="panel-body">' +
                        '<div class="text-muted">Context</div><strong>' +
                        (evaluation.context_complete ? "Complete" : "Partial") +
                        '</strong></div></div></div>' +
                '</div>'
        }

        function equalizeEvaluationSummaryCards(containerSelector) {
            var bodies = $(containerSelector).find('.ai-evaluation-card .panel-body')
            var maxHeight = 0

            bodies.css('min-height', '')
            bodies.each(function () {
                maxHeight = Math.max(maxHeight, $(this).outerHeight())
            })

            if (maxHeight > 0) {
                bodies.css('min-height', Math.ceil(maxHeight) + 'px')
            }
        }

        function renderMissingContext(evaluation) {
            var missing = evaluation && evaluation.missing_context ? evaluation.missing_context : []

            if (!missing.length) {
                $("#evaluationMissingContext").empty()
                return
            }

            var html = '<div class="alert alert-warning"><strong>Missing context:</strong><ul style="margin-bottom:0;">'
            $.each(missing, function (i, field) {
                html += "<li>" + escapeHtml(humanizeIdentifier(field)) + "</li>"
            })
            html += "</ul></div>"
            $("#evaluationMissingContext").html(html)
        }

        function renderSuggestions(suggestions) {
            if (!suggestions || !suggestions.length) {
                $("#evaluationContextSuggestions").empty()
                return
            }

            var html = '<div class="alert alert-info"><strong>Context change suggestions:</strong><ul style="margin-bottom:0;">'
            $.each(suggestions, function (i, suggestion) {
                html += "<li>" + escapeHtml(suggestion) + "</li>"
            })
            html += "</ul></div>"
            $("#evaluationContextSuggestions").html(html)
        }

        function renderCueTable(evaluation, targetSelector) {
            var results = evaluation && evaluation.cues && evaluation.cues.results ?
                evaluation.cues.results : []
            var html = ""

            $.each(results, function (i, result) {
                if (!result || result.max_count <= 0) {
                    return
                }

                var evidence = result.evidence && result.evidence.length ?
                    result.evidence.join("; ") :
                    (result.min_count !== result.max_count ? "Insufficient context to resolve exactly." : "-")

                html += "<tr>" +
                    "<td>" + escapeHtml(humanizeIdentifier(result.id)) + "</td>" +
                    "<td>" + escapeHtml(valueRange(result.min_count, result.max_count)) + "</td>" +
                    "<td>" + escapeHtml(humanizeIdentifier(result.source)) + "</td>" +
                    "<td>" + escapeHtml(evidence) + "</td>" +
                    "</tr>"
            })

            if (!html) {
                html = '<tr><td colspan="4" class="text-muted">No positive or unresolved cues.</td></tr>'
            }

            $(targetSelector || "#evaluationCueTableBody").html(html)
        }

        function renderPremiseTable(evaluation, targetSelector) {
            var elements = evaluation && evaluation.premise_alignment &&
                evaluation.premise_alignment.elements ?
                evaluation.premise_alignment.elements : []
            var html = ""

            $.each(elements, function (i, element) {
                var score = element.score === undefined || element.score === null ?
                    "Unknown" :
                    String(element.score)

                html += "<tr>" +
                    "<td>" + escapeHtml(humanizeIdentifier(element.id)) + "</td>" +
                    "<td>" + escapeHtml(score) + "</td>" +
                    "<td>" + escapeHtml(element.explanation || "-") + "</td>" +
                    "</tr>"
            })

            if (!html) {
                html = '<tr><td colspan="3" class="text-muted">No premise-alignment details.</td></tr>'
            }

            $(targetSelector || "#evaluationPremiseTableBody").html(html)
        }

        function adjustmentStatusHtml(status, iterations) {
            var css = "alert-info"
            var title = "Evaluation complete."

            if (status === "reached") {
                css = "alert-success"
                title = "Target difficulty reached."
            } else if (status === "possible_unconfirmed") {
                css = "alert-warning"
                title = "Target difficulty is possible, but unresolved evaluation ranges prevent confirmation."
            } else if (status === "not_reached") {
                css = "alert-warning"
                title = "Target difficulty was not reached with the accepted automatic revisions."
            } else if (status === "manual_revision") {
                css = "alert-info"
                title = "Manual revision applied and re-evaluated."
            }

            var iterationText = ""
            if (iterations !== undefined && iterations !== null) {
                iterationText = " Accepted automatic revisions: " + escapeHtml(String(iterations)) + "."
            }

            return '<div class="alert ' + css + '"><strong>' +
                escapeHtml(title) + "</strong>" + iterationText + "</div>"
        }

        function renderGenerationResult(response) {
            generatedTemplate = response.email || response
            generatedEvaluation = response.evaluation || null

            $("#ai_result_subject").val(generatedTemplate.subject || "")
            $("#ai_result_text").val(generatedTemplate.text || "")
            $("#ai_result_target_difficulty").text(
                labelDifficulty(generationContext ? generationContext.target_difficulty : "")
            )
            $("#ai_result_actual_difficulty").text(
                generatedEvaluation ? difficultyText(generatedEvaluation.difficulty) : "-"
            )

            var preview = document.getElementById("ai_result_html")
            if (preview) {
                preview.srcdoc = generatedTemplate.html || ""
            }

            $("#aiGenerationEvaluationSummary").html(
                generatedEvaluation ? evaluationSummaryHtml(generatedEvaluation) : ""
            )

            $("#aiAdjustmentMeta").html(
                response.status ?
                    adjustmentStatusHtml(response.status, response.iterations) :
                    ""
            )

            if (response.context_change_suggestions && response.context_change_suggestions.length) {
                var suggestionHtml = '<div class="alert alert-info"><strong>Suggestions:</strong><ul style="margin-bottom:0;">'
                $.each(response.context_change_suggestions, function (i, suggestion) {
                    suggestionHtml += "<li>" + escapeHtml(suggestion) + "</li>"
                })
                suggestionHtml += "</ul></div>"
                $("#aiAdjustmentMeta").append(suggestionHtml)
            }

            refreshAttachmentSummaries()
            renderCueTable(generatedEvaluation, "#aiGenerationCueTableBody")
            renderPremiseTable(generatedEvaluation, "#aiGenerationPremiseTableBody")
            $("#aiGenerationEvaluationDetails").removeClass("in").attr("aria-expanded", "false")

            showAIState("result")

            setTimeout(function () {
                equalizeEvaluationSummaryCards("#aiGenerationEvaluationSummary")
            }, 0)
        }

        function showEvaluationResultLoading() {
            $("#evaluationResultFlashes").empty()
            $("#evaluationResultContent").hide()
            $("#evaluationResultLoading").show()
            $("#evaluationResultCloseButton, #evaluationResultFooter").hide()
            $("#reevaluateEvaluatedEmailButton, #reviseEvaluatedEmailButton, #changeDifficultyButton, #applyEvaluatedEmailButton").hide()
            $("#EvaluationResultModal").modal({
                backdrop: "static",
                keyboard: false,
                show: true
            })
        }

        function renderEvaluationResult(evaluation, email, emailChanged, adjustmentResult) {
            currentEvaluation = evaluation
            evaluatedEmailChanged = !!emailChanged

            $("#evaluationResultLoading").hide()
            $("#evaluationResultFlashes").empty()
            $("#evaluationResultContent").show()
            $("#evaluationResultCloseButton, #evaluationResultFooter").show()

            if (email) {
                $("#evaluation_revised_subject").val(email.subject || "")
                $("#evaluation_revised_text").val(email.text || "")
                var preview = document.getElementById("evaluation_revised_html")
                if (preview) {
                    preview.srcdoc = email.html || ""
                }
                $("#evaluationRevisedEmailPreview").show()
            } else {
                $("#evaluationRevisedEmailPreview").hide()
            }

            refreshAttachmentSummaries()
            $("#evaluationResultSummary").html(evaluationSummaryHtml(evaluation))
            renderMissingContext(evaluation)
            renderCueTable(evaluation)
            renderPremiseTable(evaluation)
            $("#evaluationResultDetails").removeClass("in").attr("aria-expanded", "false")

            if (adjustmentResult && adjustmentResult.status) {
                $("#evaluationResultMeta").html(
                    adjustmentStatusHtml(adjustmentResult.status, adjustmentResult.iterations)
                )
                renderSuggestions(adjustmentResult.context_change_suggestions || [])
            } else {
                $("#evaluationResultMeta").empty()
                $("#evaluationContextSuggestions").empty()
            }

            if (emailChanged && email) {
                $("#applyEvaluatedEmailButton").show()
            } else {
                $("#applyEvaluatedEmailButton").hide()
            }

            $("#reevaluateEvaluatedEmailButton, #reviseEvaluatedEmailButton, #changeDifficultyButton").show()

            setTimeout(function () {
                equalizeEvaluationSummaryCards("#evaluationResultSummary")
            }, 0)
        }

        function evaluateInput(input, emailChanged, adjustmentResult) {
            showEvaluationResultLoading()
            // Every deliberate Evaluate action requests a new model judgment.
            // Keep the cache for automatic attachment changes only.
            var request = cloneObject(input)
            request.evaluation_mode = "refresh"
            query("/ai/templates/evaluate", "POST", request, true)
                .success(function (response) {
                    currentEvaluationInput = input
                    renderEvaluationResult(response, input.email, emailChanged, adjustmentResult)
                })
                .error(function (response) {
                    showEvaluationResultError(
                        responseMessage(response, "Failed to evaluate email.")
                    )
                })
        }

        function startGenerateAndAdjust() {
            generationContext = pendingGenerationContext
            evaluationContext = buildEvaluationContext()

            $("#EvaluationContextModal").modal("hide")
            $("#aiModalFlashes").empty()
            $("#aiLoadingText").text("Generating, evaluating, and adjusting email...")
            showAIState("loading")

            query("/ai/templates/generate-and-adjust", "POST", {
                generation_context: generationContext,
                evaluation_context: evaluationContext,
                max_iterations: 3
            }, true)
                .success(function (response) {
                    renderGenerationResult(response)
                })
                .error(function (response) {
                    showAIState("form")
                    showAIError(
                        responseMessage(response, "Failed to generate and evaluate email.")
                    )
                })
        }

        function startManualEvaluation() {
            var input = {
                email: getCurrentEmail(),
                generation_context: getManualEvaluationGenerationContext(),
                evaluation_context: buildEvaluationContext()
            }

            evaluationContext = input.evaluation_context
            currentEvaluationInput = input

            $("#EvaluationContextModal").modal("hide")
            evaluateInput(input, false, null)
        }

        $("#ai_scenario").on("change", function () {
            if ($(this).val() === "custom") {
                $("#ai_custom_scenario_group").show()
            } else {
                $("#ai_custom_scenario_group").hide()
                $("#ai_custom_scenario").val("")
            }
        })

        $("#evaluation_manual_scenario").on("change", updateManualScenarioFields)
        $("#evaluation_link_usage").on("change", updateLinkFields)
        $("#evaluation_sending_profile").on("change", function () {
            updateSendingProfilePreview()

            var profile = findSendingProfile($(this).val())
            if (!profile) {
                return
            }

            var parsed = parseMailbox(profile.from_address)
            if (parsed && parsed.display_name &&
                !$.trim($("#evaluation_simulated_sender_name").val())) {
                $("#evaluation_simulated_sender_name").val(parsed.display_name)
            }
        })

        $("#ai_no_attachments, #evaluation_no_attachments").on("change", function () {
            setExplicitNoAttachments($(this).prop("checked"))
        })

        $("#aiAttachmentUpload, #evaluationAttachmentUpload, #aiResultAttachmentUpload, #evaluationResultAttachmentUpload").on("click", function () {
            this.value = null
        }).on("change", function () {
            if (this.files && this.files.length) {
                attachmentsExplicitNone = false
                attach(this.files)
                syncNoAttachmentCheckboxes()
            }
        })

        $("#aiAttachmentsSummary, #evaluationAttachmentsSummary, #aiResultAttachmentsSummary, #evaluationResultAttachmentsSummary").on(
            "click",
            ".ai-remove-attachment",
            function () {
                var rowIndex = parseInt($(this).attr("data-attachment-row"), 10)
                if (!isNaN(rowIndex)) {
                    removeSharedAttachment(rowIndex)
                }
            }
        )

        $("#aiGenerateButton").on("click", function () {
            $("#aiModalFlashes").empty()
            openEvaluationContext("generate")
        })

        $("#evaluateEmailButton").on("click", function () {
            openEvaluationContext("evaluate")
        })

        $("#aiReevaluateButton").on("click", function () {
            refreshResult("generate")
        })

        $("#reevaluateEvaluatedEmailButton").on("click", function () {
            refreshResult("evaluate")
        })

        $("#evaluationContextSubmitButton").on("click", function () {
            $("#evaluationContextFlashes").empty()

            if (evaluationFlowMode === "generate") {
                startGenerateAndAdjust()
            } else if (evaluationFlowMode === "evaluate") {
                startManualEvaluation()
            } else {
                showEvaluationContextError("Evaluation flow is not initialized.")
            }
        })

        $("#aiBackToParametersButton").on("click", function () {
            $("#aiModalFlashes").empty()
            showAIState("form")
        })

        function revisionState(mode) {
            if (mode === "generate") {
                if (!generatedTemplate || !generationContext || !evaluationContext) {
                    return null
                }
                return {
                    email: generatedTemplate,
                    generation_context: generationContext,
                    evaluation_context: evaluationContext,
                    evaluation: generatedEvaluation
                }
            }

            if (!currentEvaluationInput || !currentEvaluation) {
                return null
            }

            return {
                email: currentEvaluationInput.email,
                generation_context: currentEvaluationInput.generation_context || {},
                evaluation_context: currentEvaluationInput.evaluation_context || {},
                evaluation: currentEvaluation
            }
        }

        function showResultError(mode, message) {
            if (mode === "generate") {
                showAIError(message)
            } else {
                $("#evaluationResultFlashes").html(
                    '<div class="alert alert-danger">' + escapeHtml(message) + "</div>"
                )
            }
        }

        function openRevisionModal(mode) {
            if (!revisionState(mode)) {
                showResultError(mode, "There is no email to revise.")
                return
            }

            revisionFlowMode = mode
            $("#emailRevisionFlashes").empty()
            $("#email_revision_feedback").val("")
            $("#EmailRevisionModal").modal({
                backdrop: "static",
                keyboard: false,
                show: true
            })

            setTimeout(function () {
                $("#email_revision_feedback").focus()
            }, 0)
        }

        function revisionRequest(state, feedback) {
            var context = state.generation_context || {}
            return {
                email: normalizedEmail(state.email),
                feedback: feedback,
                target_audience: context.target_audience || "",
                recipient_role: context.recipient_role || "",
                organization_context: context.organization_context || "",
                sender_context: context.sender_context || "",
                scenario: context.scenario || "",
                custom_scenario: context.custom_scenario || "",
                language: context.language || "",
                target_difficulty: context.target_difficulty || "",
                evaluation_context: state.evaluation_context || {}
            }
        }

        function finishManualRevision(mode, state, revisedEmail, evaluation) {
            if (mode === "generate") {
                renderGenerationResult({
                    email: revisedEmail,
                    evaluation: evaluation,
                    status: "manual_revision",
                    iterations: 0
                })
                return
            }

            currentEvaluationInput.email = revisedEmail
            currentEvaluation = evaluation
            renderEvaluationResult(
                evaluation,
                revisedEmail,
                emailDiffersFromTemplate(revisedEmail),
                {
                    status: "manual_revision",
                    iterations: 0
                }
            )
        }

        $("#aiReviseButton").on("click", function () {
            openRevisionModal("generate")
        })

        $("#reviseEvaluatedEmailButton").on("click", function () {
            openRevisionModal("evaluate")
        })

        $("#emailRevisionSubmitButton").on("click", function () {
            var mode = revisionFlowMode
            var state = revisionState(mode)
            var feedback = $.trim($("#email_revision_feedback").val() || "")
            var button = $(this)

            if (!feedback) {
                showEmailRevisionError("Please describe what should be changed.")
                return
            }

            if (!state) {
                showEmailRevisionError("There is no email to revise.")
                return
            }

            $("#emailRevisionFlashes").empty()
            button.prop("disabled", true)

            $("#EmailRevisionModal").one("hidden.bs.modal", function () {
                $("#EmailRevisionLoadingModal").modal({
                    backdrop: "static",
                    keyboard: false,
                    show: true
                })

                query("/ai/templates/revise", "POST", revisionRequest(state, feedback), true)
                    .success(function (revisedEmail) {
                        query("/ai/templates/evaluate", "POST", {
                            email: revisedEmail,
                            generation_context: state.generation_context || {},
                            evaluation_context: state.evaluation_context || {}
                        }, true)
                            .success(function (evaluation) {
                                finishManualRevision(mode, state, revisedEmail, evaluation)
                                $("#EmailRevisionLoadingModal").modal("hide")
                            })
                            .error(function (response) {
                                var message = responseMessage(
                                    response,
                                    "Email was revised, but re-evaluation failed."
                                )
                                $("#EmailRevisionLoadingModal")
                                    .one("hidden.bs.modal", function () {
                                        showResultError(mode, message)
                                    })
                                    .modal("hide")
                            })
                    })
                    .error(function (response) {
                        var message = responseMessage(response, "Failed to revise email.")
                        $("#EmailRevisionLoadingModal")
                            .one("hidden.bs.modal", function () {
                                showResultError(mode, message)
                            })
                            .modal("hide")
                    })
                    .always(function () {
                        button.prop("disabled", false)
                    })
            })

            $("#EmailRevisionModal").modal("hide")
        })

        $("#aiConfirmButton").on("click", function () {
            if (!generatedTemplate) {
                showAIError("There is no generated email to confirm.")
                return
            }

            applyEmailToTemplate(generatedTemplate)
            lastAppliedGenerationContext = cloneObject(generationContext)
            lastAppliedEvaluationContext = cloneObject(evaluationContext)
            $("#GenerateWithAIModal").modal("hide")
        })

        function difficultyState(mode) {
            if (mode === "generate") {
                if (!generatedTemplate || !generationContext || !evaluationContext || !generatedEvaluation) {
                    return null
                }
                return {
                    input: {
                        email: normalizedEmail(generatedTemplate),
                        generation_context: cloneObject(generationContext),
                        evaluation_context: cloneObject(evaluationContext)
                    },
                    evaluation: generatedEvaluation
                }
            }

            if (!currentEvaluationInput || !currentEvaluation) {
                return null
            }

            return {
                input: cloneObject(currentEvaluationInput),
                evaluation: currentEvaluation
            }
        }

        function openChangeDifficultyModal(mode) {
            var state = difficultyState(mode)
            if (!state) {
                showResultError(mode, "There is no evaluated email to adjust.")
                return
            }

            difficultyFlowMode = mode
            $("#changeDifficultyFlashes").empty()

            if (state.evaluation.difficulty &&
                state.evaluation.difficulty.resolved &&
                state.evaluation.difficulty.detection_difficulty) {
                $("#change_difficulty_target").val(
                    state.evaluation.difficulty.detection_difficulty
                )
            }

            $("#ChangeDifficultyModal").modal({
                backdrop: "static",
                keyboard: false,
                show: true
            })
        }

        $("#aiChangeDifficultyButton").on("click", function () {
            openChangeDifficultyModal("generate")
        })

        $("#changeDifficultyButton").on("click", function () {
            openChangeDifficultyModal("evaluate")
        })

        $("#changeDifficultySubmitButton").on("click", function () {
            var mode = difficultyFlowMode
            var state = difficultyState(mode)
            if (!state) {
                showChangeDifficultyError("There is no evaluated email.")
                return
            }

            var target = $("#change_difficulty_target").val()
            var feedback = $("#change_difficulty_feedback").val() || ""
            var iterations = parseInt($("#change_difficulty_iterations").val(), 10) || 3
            var button = $(this)

            $("#changeDifficultyFlashes").empty()
            button.prop("disabled", true)

            $("#ChangeDifficultyModal").one("hidden.bs.modal", function () {
                $("#DifficultyAdjustmentLoadingModal").modal({
                    backdrop: "static",
                    keyboard: false,
                    show: true
                })

                query("/ai/templates/change-difficulty", "POST", {
                    input: state.input,
                    target: target,
                    user_feedback: feedback,
                    max_iterations: iterations
                }, true)
                    .success(function (response) {
                        if (mode === "generate") {
                            generationContext.target_difficulty = target
                            renderGenerationResult(response)
                        } else {
                            currentEvaluationInput.email = response.email
                            if (currentEvaluationInput.generation_context) {
                                currentEvaluationInput.generation_context.target_difficulty = target
                            }
                            currentEvaluation = response.evaluation
                            renderEvaluationResult(
                                response.evaluation,
                                response.email,
                                emailDiffersFromTemplate(response.email),
                                response
                            )
                        }

                        $("#DifficultyAdjustmentLoadingModal").modal("hide")
                    })
                    .error(function (response) {
                        var message = responseMessage(
                            response,
                            "Failed to change email difficulty."
                        )

                        $("#DifficultyAdjustmentLoadingModal")
                            .one("hidden.bs.modal", function () {
                                $("#ChangeDifficultyModal").modal({
                                    backdrop: "static",
                                    keyboard: false,
                                    show: true
                                })
                                showChangeDifficultyError(message)
                            })
                            .modal("hide")
                    })
                    .always(function () {
                        button.prop("disabled", false)
                    })
            })

            $("#ChangeDifficultyModal").modal("hide")
        })

        $("#applyEvaluatedEmailButton").on("click", function () {
            if (!evaluatedEmailChanged || !currentEvaluationInput) {
                return
            }

            applyEmailToTemplate(currentEvaluationInput.email)
            lastAppliedGenerationContext = cloneObject(currentEvaluationInput.generation_context)
            lastAppliedEvaluationContext = cloneObject(currentEvaluationInput.evaluation_context)
            evaluatedEmailChanged = false
            $("#applyEvaluatedEmailButton").hide()

            $("#modal\\.flashes").html(
                '<div class="alert alert-success">Revised email applied to the current template.</div>'
            )
            $("#EvaluationResultModal").modal("hide")
        })

        $("#GenerateWithAIModal").on("shown.bs.modal", function () {
            refreshAttachmentSummaries()
        })

        $("#GenerateWithAIModal").on("hidden.bs.modal", function () {
            $("#aiModalFlashes").empty()

            var preview = document.getElementById("ai_result_html")
            if (preview) {
                preview.srcdoc = ""
            }
            $("#aiGenerationCueTableBody, #aiGenerationPremiseTableBody").empty()
            $("#aiGenerationEvaluationDetails").removeClass("in")

            generationContext = null
            evaluationContext = null
            generatedTemplate = null
            generatedEvaluation = null
            pendingGenerationContext = null
            evaluationFlowMode = null
            showAIState("form")
        })

        $("#EvaluationResultModal").on("hidden.bs.modal", function () {
            $("#evaluationResultFlashes").empty()
            $("#evaluationResultMeta").empty()
            $("#evaluationResultSummary").empty()
            $("#evaluationMissingContext").empty()
            $("#evaluationContextSuggestions").empty()
            $("#evaluationCueTableBody").empty()
            $("#evaluationPremiseTableBody").empty()
            $("#evaluationResultDetails").removeClass("in")

            var preview = document.getElementById("evaluation_revised_html")
            if (preview) {
                preview.srcdoc = ""
            }
        })

        $("#EmailRevisionModal").on("hidden.bs.modal", function () {
            $("#emailRevisionFlashes").empty()
            $("#email_revision_feedback").val("")
        })

        $("#ChangeDifficultyModal").on("hidden.bs.modal", function () {
            $("#changeDifficultyFlashes").empty()
            $("#change_difficulty_feedback").val("")
        })


        $(window).on("resize.aiEvaluationSummary", function () {
            equalizeEvaluationSummaryCards("#aiGenerationEvaluationSummary")
            equalizeEvaluationSummaryCards("#evaluationResultSummary")
        })

        refreshAttachmentSummaries()
        updateLinkFields()
        updateManualScenarioFields()
        showAIState("form")
    })()

    load()

})
