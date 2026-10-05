// Theme toggle
(function () {
  var stored = localStorage.getItem("am-theme");
  var prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  if (stored === "dark" || (!stored && prefersDark)) {
    document.documentElement.setAttribute("data-theme", "dark");
  }
})();

document.addEventListener("DOMContentLoaded", function () {
  // Theme toggle button
  document
    .getElementById("theme-toggle")
    .addEventListener("click", function () {
      var isDark =
        document.documentElement.getAttribute("data-theme") === "dark";
      if (isDark) {
        document.documentElement.removeAttribute("data-theme");
        localStorage.setItem("am-theme", "light");
      } else {
        document.documentElement.setAttribute("data-theme", "dark");
        localStorage.setItem("am-theme", "dark");
      }
    });

  var state = {
    requirements: JSON.parse(
      document.getElementById("data-requirements").textContent,
    ),
    items: JSON.parse(document.getElementById("data-items").textContent),
    schema: JSON.parse(document.getElementById("data-schema").textContent),
    final: JSON.parse(document.getElementById("data-final").textContent),
    history:
      JSON.parse(document.getElementById("data-history").textContent) || [],
  };

  var isStatic = window.location.protocol === "file:";

  if (isStatic) {
    document
      .querySelectorAll("#load-btn, #export-dropdown, #print-btn")
      .forEach(function (el) {
        el.style.display = "none";
      });
  }

  // --- Helpers ---

  function getOptionTitle(optId) {
    var opt = state.schema.design_options.find(function (o) {
      return o.id === optId;
    });
    return opt ? opt.title : optId;
  }

  function computeOptionStatus(optId) {
    var opt = state.schema.design_options.find(function (o) {
      return o.id === optId;
    });
    if (!opt) return "possible";

    var checkedReqs = {};
    state.requirements.forEach(function (r) {
      checkedReqs[r.id] = r.checked;
    });

    var hardReqs = {};
    state.schema.requirements.forEach(function (r) {
      hardReqs[r.id] = r.is_hard;
    });

    var activeKeys = {};
    state.items.forEach(function (item) {
      if (item.kind === "choice" && item.answer) {
        activeKeys[item.id + "." + item.answer] = true;
      }
    });

    var reqsMet = opt.requirements_met || {};
    for (var reqId in reqsMet) {
      var status = reqsMet[reqId];
      if (
        hardReqs[reqId] &&
        checkedReqs[reqId] &&
        !status.met &&
        !status.partial
      ) {
        return "eliminated";
      }
    }

    var blocks = opt.blocks || [];
    for (var i = 0; i < blocks.length; i++) {
      var allMatch = blocks[i].condition.every(function (key) {
        return activeKeys[key];
      });
      if (allMatch) return "blocked";
    }

    return "possible";
  }

  function showConfirmModal(title, body) {
    return new Promise(function (resolve) {
      var overlay = document.getElementById("confirm-modal");
      document.getElementById("confirm-modal-title").textContent = title;
      document.getElementById("confirm-modal-body").textContent = body;
      overlay.style.display = "";

      function cleanup(result) {
        overlay.style.display = "none";
        confirmBtn.removeEventListener("click", onConfirm);
        cancelBtn.removeEventListener("click", onCancel);
        overlay.removeEventListener("click", onOverlay);
        document.removeEventListener("keydown", onEscape);
        resolve(result);
      }

      var confirmBtn = document.getElementById("confirm-modal-confirm");
      var cancelBtn = document.getElementById("confirm-modal-cancel");

      function onConfirm() {
        cleanup(true);
      }
      function onCancel() {
        cleanup(false);
      }
      function onOverlay(e) {
        if (e.target === overlay) cleanup(false);
      }

      function onEscape(e) {
        if (e.key === "Escape") cleanup(false);
      }

      confirmBtn.addEventListener("click", onConfirm);
      cancelBtn.addEventListener("click", onCancel);
      overlay.addEventListener("click", onOverlay);
      document.addEventListener("keydown", onEscape);
    });
  }

  function checkDecisionAndApply(applyFn, revertFn) {
    applyFn();

    if (!state.final) {
      render();
      syncState(0);
      return;
    }

    var newStatus = computeOptionStatus(state.final.option);
    if (newStatus !== "eliminated" && newStatus !== "blocked") {
      render();
      syncState(0);
      return;
    }

    var title = getOptionTitle(state.final.option);
    revertFn();

    showConfirmModal(
      "Clear decision?",
      'This change would make "' +
        title +
        '" ' +
        newStatus +
        ". Your decision will be cleared if you continue.",
    ).then(function (confirmed) {
      if (!confirmed) return;
      applyFn();
      if (state.final) {
        state.history.push({
          decided_on: new Date().toISOString(),
          present: state.final.present || [],
          option: state.final.option,
          title: getOptionTitle(state.final.option),
          rationale: state.final.rationale || "",
        });
      }
      state.final = null;
      syncFinalize(null);
      render();
      syncState(0);
    });
  }

  // --- Event handlers: mutate state, then render ---

  document.querySelectorAll(".req").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var id = this.dataset.id;
      var r = state.requirements.find(function (req) {
        return req.id === id;
      });
      if (!r) return;

      checkDecisionAndApply(
        function () {
          r.checked = !r.checked;
        },
        function () {
          r.checked = !r.checked;
        },
      );
    });
  });

  document.querySelectorAll(".choice").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var itemId = this.dataset.item;
      var answerId = this.dataset.answer;
      var item = state.items.find(function (it) {
        return it.id === itemId;
      });
      if (!item) return;

      var prevAnswer = item.answer;
      checkDecisionAndApply(
        function () {
          item.answer = item.answer === answerId ? null : answerId;
        },
        function () {
          item.answer = prevAnswer;
        },
      );
    });
  });

  document.querySelectorAll(".item-input").forEach(function (input) {
    input.addEventListener("input", function () {
      var itemId = this.dataset.item;
      var item = state.items.find(function (it) {
        return it.id === itemId;
      });
      if (item) {
        if (this.type === "number") {
          item.value = this.value === "" ? null : Number(this.value);
        } else {
          item.value = this.value || null;
        }
      }
      syncState(500);
    });
  });

  // Option list selection (master-detail, UI-only)
  document.querySelectorAll(".option-list-item").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var optId = this.dataset.optionId;
      document.querySelectorAll(".option-list-item").forEach(function (b) {
        var isCurrent = b.dataset.optionId === optId;
        b.classList.toggle("active", isCurrent);
        if (isCurrent) {
          b.setAttribute("aria-current", "true");
        } else {
          b.removeAttribute("aria-current");
        }
      });
      document.querySelectorAll(".option-detail-panel").forEach(function (p) {
        p.classList.toggle("active", p.id === "option-" + optId);
      });
    });
  });

  // --- Pick / Finalize handlers ---

  document.querySelectorAll(".pick-btn").forEach(function (btn) {
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      var area = this.closest(".pick-area");
      area.querySelector(".pick-form").style.display = "";
      this.style.display = "none";
    });
  });

  document.querySelectorAll(".pick-cancel").forEach(function (btn) {
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      var area = this.closest(".pick-area");
      area.querySelector(".pick-form").style.display = "none";
      area.querySelector(".pick-btn").style.display = "";
      area.querySelector(".pick-rationale").value = "";
    });
  });

  document.querySelectorAll(".pick-confirm").forEach(function (btn) {
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      var area = this.closest(".pick-area");
      var optId = area.dataset.option;
      var rationale = area.querySelector(".pick-rationale").value.trim();
      var presentRaw = area.querySelector(".pick-present").value.trim();
      var present = presentRaw
        ? presentRaw
            .split(",")
            .map(function (s) {
              return s.trim();
            })
            .filter(Boolean)
        : [];

      // Move existing decision to history before replacing
      if (state.final) {
        state.history.push({
          decided_on: new Date().toISOString(),
          present: state.final.present || [],
          option: state.final.option,
          title: getOptionTitle(state.final.option),
          rationale: state.final.rationale || "",
        });
      }
      state.final = { option: optId, rationale: rationale, present: present };
      syncFinalize(state.final);
      render();
      document
        .getElementById("decision-section")
        .scrollIntoView({ behavior: "smooth", block: "start" });
    });
  });

  var undoBtn = document.getElementById("decision-undo");
  if (undoBtn) {
    undoBtn.addEventListener("click", function () {
      if (state.final) {
        state.history.push({
          decided_on: new Date().toISOString(),
          present: state.final.present || [],
          option: state.final.option,
          title: getOptionTitle(state.final.option),
          rationale: state.final.rationale || "",
        });
      }
      state.final = null;
      syncFinalize(null);
      render();
    });
  }

  // --- Render: sync ALL UI from state ---

  // --- Conditional visibility (show_when) ---

  // Build show_when lookup from DOM data attributes
  var showWhenMap = {};
  document.querySelectorAll(".item[data-show-when]").forEach(function (el) {
    var itemId =
      el.querySelector(".choice") && el.querySelector(".choice").dataset.item;
    if (!itemId) {
      var input = el.querySelector(".item-input");
      if (input) itemId = input.dataset.item;
    }
    if (!itemId) {
      // fallback: extract from the input/choice id attribute
      var labelFor = el.querySelector("label");
      if (labelFor) {
        var forAttr = labelFor.getAttribute("for");
        if (forAttr && forAttr.startsWith("input-")) {
          itemId = forAttr.substring(6);
        }
      }
    }
    if (itemId) {
      showWhenMap[itemId] = JSON.parse(el.dataset.showWhen);
    }
  });

  function computeVisibleItems() {
    var activeKeys = {};
    state.items.forEach(function (item) {
      if (item.kind === "choice" && item.answer) {
        activeKeys[item.id + "." + item.answer] = true;
      }
    });

    var visible = {};
    state.items.forEach(function (item) {
      if (!showWhenMap[item.id]) {
        visible[item.id] = true;
      }
    });

    var changed = true;
    while (changed) {
      changed = false;
      for (var itemId in showWhenMap) {
        var groups = showWhenMap[itemId];
        var wasVisible = !!visible[itemId];
        var nowVisible = groups.some(function (group) {
          return group.every(function (key) {
            var parts = key.split(".");
            return visible[parts[0]] && activeKeys[key];
          });
        });
        if (nowVisible !== wasVisible) {
          visible[itemId] = nowVisible;
          changed = true;
        }
      }
    }
    return visible;
  }

  function applyVisibility() {
    var visible = computeVisibleItems();
    // Clear answers for hidden items
    state.items.forEach(function (item) {
      if (!visible[item.id] && showWhenMap[item.id]) {
        if (item.kind === "choice") item.answer = null;
        else item.value = null;
      }
    });
    // Toggle DOM visibility
    document.querySelectorAll(".item[data-show-when]").forEach(function (el) {
      var itemId = null;
      var choice = el.querySelector(".choice");
      if (choice) itemId = choice.dataset.item;
      if (!itemId) {
        var input = el.querySelector(".item-input");
        if (input) itemId = input.dataset.item;
      }
      if (!itemId) {
        var labelFor = el.querySelector("label");
        if (labelFor) {
          var forAttr = labelFor.getAttribute("for");
          if (forAttr && forAttr.startsWith("input-"))
            itemId = forAttr.substring(6);
        }
      }
      if (itemId) {
        el.style.display = visible[itemId] ? "" : "none";
      }
    });
  }

  function render() {
    // Evaluate conditional item visibility
    applyVisibility();

    // Sync requirement buttons
    document.querySelectorAll(".req").forEach(function (btn) {
      var r = state.requirements.find(function (req) {
        return req.id === btn.dataset.id;
      });
      if (r) {
        btn.classList.toggle("active", r.checked);
        btn.setAttribute("aria-pressed", r.checked);
      }
    });

    // Sync choice buttons
    document.querySelectorAll(".choice").forEach(function (btn) {
      var item = state.items.find(function (it) {
        return it.id === btn.dataset.item;
      });
      var isSelected = item && item.answer === btn.dataset.answer;
      btn.classList.toggle("selected", isSelected);
      btn.setAttribute("aria-pressed", isSelected);
    });

    // Sync text/number inputs (skip if focused to avoid clobbering mid-type)
    document.querySelectorAll(".item-input").forEach(function (input) {
      if (document.activeElement === input) return;
      var item = state.items.find(function (it) {
        return it.id === input.dataset.item;
      });
      if (item) {
        input.value = item.value != null ? item.value : "";
      }
    });

    // Evaluate design options
    var checkedReqs = {};
    state.requirements.forEach(function (r) {
      checkedReqs[r.id] = r.checked;
    });

    var hardReqs = {};
    state.schema.requirements.forEach(function (r) {
      hardReqs[r.id] = r.is_hard;
    });

    var activeKeys = {};
    state.items.forEach(function (item) {
      if (item.kind === "choice" && item.answer) {
        activeKeys[item.id + "." + item.answer] = true;
      }
    });

    state.schema.design_options.forEach(function (opt) {
      var hardMet = 0,
        hardOf = 0,
        softMet = 0,
        softOf = 0;
      var failedReqs = [];

      var reqsMet = opt.requirements_met || {};
      Object.keys(reqsMet).forEach(function (reqId) {
        var status = reqsMet[reqId];
        var isHard = hardReqs[reqId];
        var checked = checkedReqs[reqId];

        if (isHard) {
          hardOf++;
          if (status.met || status.partial) {
            hardMet++;
          } else if (checked) {
            failedReqs.push(reqId);
          }
        } else {
          softOf++;
          if (status.met || status.partial) {
            softMet++;
          }
        }
      });

      var firedBlocks = [];
      (opt.blocks || []).forEach(function (block) {
        var allMatch = block.condition.every(function (key) {
          return activeKeys[key];
        });
        if (allMatch) {
          firedBlocks.push(block.reason);
        }
      });

      var effects = [];
      Object.keys(opt.effects || {}).forEach(function (key) {
        if (!activeKeys[key]) return;
        var parts = key.split(".");
        (opt.effects[key] || []).forEach(function (eff) {
          effects.push({
            item: parts[0],
            answer: parts[1],
            kind: eff.kind,
            category: eff.category || "",
            text: eff.text,
          });
        });
      });

      var optStatus;
      if (failedReqs.length > 0) {
        optStatus = "eliminated";
      } else if (firedBlocks.length > 0) {
        optStatus = "blocked";
      } else if (state.final && state.final.option === opt.id) {
        optStatus = "picked";
      } else {
        optStatus = "possible";
      }

      // Update list item
      var listItem = document.querySelector(
        '.option-list-item[data-option-id="' + opt.id + '"]',
      );
      if (listItem) {
        var wasActive = listItem.classList.contains("active");
        listItem.className =
          "option-list-item " + optStatus + (wasActive ? " active" : "");
        var listStatus = listItem.querySelector(".option-status");
        listStatus.textContent = optStatus;
        listStatus.className = "option-status " + optStatus;

        var listGrid = listItem.querySelector(".req-grid");
        if (listGrid) {
          updateReqGrid(
            listGrid,
            reqsMet,
            hardReqs,
            checkedReqs,
            hardMet,
            hardOf,
            softMet,
            softOf,
          );
        }
      }

      // Update detail panel
      var el = document.getElementById("option-" + opt.id);
      if (!el) return;

      var wasActive = el.classList.contains("active");
      el.className =
        "option-detail-panel " + optStatus + (wasActive ? " active" : "");

      var statusEl = el.querySelector(".option-status");
      statusEl.textContent = optStatus;
      statusEl.className = "option-status " + optStatus;

      var failedEl = el.querySelector(".failed-reqs");
      if (failedReqs.length > 0) {
        var descs = failedReqs.map(function (id) {
          var r = state.schema.requirements.find(function (req) {
            return req.id === id;
          });
          return r ? r.description : id;
        });
        failedEl.textContent = "Failed: " + descs.join(", ");
        failedEl.style.display = "";
      } else {
        failedEl.style.display = "none";
      }

      var blocksEl = el.querySelector(".blocks-list");
      if (firedBlocks.length > 0) {
        blocksEl.innerHTML = firedBlocks
          .map(function (r) {
            return '<li class="block-reason">' + escapeHtml(r) + "</li>";
          })
          .join("");
        blocksEl.style.display = "";
      } else {
        blocksEl.style.display = "none";
      }

      var effectsEl = el.querySelector(".effects-list");
      if (effects.length > 0) {
        effectsEl.innerHTML = effects
          .map(function (e) {
            var cat = e.category
              ? '<span class="effect-category">' +
                escapeHtml(e.category) +
                "</span> "
              : "";
            return (
              "<li>" +
              cat +
              escapeHtml(e.text) +
              ' <span style="color:var(--text-tertiary)">(' +
              escapeHtml(e.kind) +
              ")</span></li>"
            );
          })
          .join("");
        effectsEl.style.display = "";
      } else {
        effectsEl.style.display = "none";
      }

      // Update pick area
      var pickArea = el.querySelector(".pick-area");
      if (pickArea) {
        var isPicked = state.final && state.final.option === opt.id;
        if (isPicked) {
          pickArea.style.display = "none";
        } else if (optStatus === "possible") {
          pickArea.style.display = "";
          var pickBtn = pickArea.querySelector(".pick-btn");
          pickBtn.style.display = "";
          pickBtn.textContent = state.final
            ? "Pick this instead"
            : "Pick this option";
          pickArea.querySelector(".pick-form").style.display = "none";
          pickArea.querySelector(".pick-rationale").value = "";
        } else {
          pickArea.style.display = "none";
        }
      }
    });

    // Update decision section
    var decisionEmpty = document.getElementById("decision-empty");
    var decisionMade = document.getElementById("decision-made");
    if (state.final) {
      document.getElementById("decision-title").textContent = getOptionTitle(
        state.final.option,
      );
      var metaEl = document.getElementById("decision-meta");
      var metaParts = [];
      if (state.final.present && state.final.present.length > 0) {
        metaParts.push(state.final.present.join(", "));
      }
      if (metaParts.length > 0) {
        metaEl.textContent = metaParts.join(" · ");
        metaEl.style.display = "";
      } else {
        metaEl.style.display = "none";
      }
      var rationaleEl = document.getElementById("decision-rationale");
      if (state.final.rationale) {
        rationaleEl.textContent = state.final.rationale;
        rationaleEl.style.display = "";
      } else {
        rationaleEl.style.display = "none";
      }
      decisionEmpty.style.display = "none";
      decisionMade.style.display = "";
    } else {
      decisionEmpty.style.display = "";
      decisionMade.style.display = "none";
    }

    // Update history section
    var historySection = document.getElementById("history-section");
    if (state.history.length > 0) {
      historySection.style.display = "";
      document.getElementById("history-count").textContent =
        "(" + state.history.length + ")";
      var historyList = document.getElementById("history-list");
      // Render newest first
      var reversed = state.history.slice().reverse();
      historyList.innerHTML = reversed
        .map(function (entry) {
          var parts = ['<div class="history-entry">'];
          parts.push(
            '<div class="history-entry-title">' +
              escapeHtml(entry.title) +
              "</div>",
          );
          var metaParts = [];
          if (entry.decided_on) {
            metaParts.push(formatDateTime(entry.decided_on));
          }
          if (entry.present && entry.present.length > 0) {
            metaParts.push(entry.present.join(", "));
          }
          if (metaParts.length > 0) {
            parts.push(
              '<div class="history-entry-meta">' +
                escapeHtml(metaParts.join(" · ")) +
                "</div>",
            );
          }
          if (entry.rationale) {
            parts.push(
              '<div class="history-entry-rationale">' +
                escapeHtml(entry.rationale) +
                "</div>",
            );
          }
          parts.push("</div>");
          return parts.join("");
        })
        .join("");
    } else {
      historySection.style.display = "none";
    }
  }

  function updateReqGrid(
    gridEl,
    reqsMet,
    hardReqs,
    checkedReqs,
    hardMet,
    hardOf,
    softMet,
    softOf,
  ) {
    gridEl.querySelectorAll(".req-cell").forEach(function (cell) {
      var reqId = cell.dataset.req;
      var reqStatus = reqsMet[reqId];
      var isHard = hardReqs[reqId];
      var checked = checkedReqs[reqId];
      cell.className = "req-cell";
      if (reqStatus) {
        if (reqStatus.met) {
          cell.classList.add("met");
        } else if (reqStatus.partial) {
          cell.classList.add("partial");
        } else if (checked && isHard) {
          cell.classList.add("failed");
        }
      }
    });
    var label = gridEl.querySelector(".req-grid-label");
    label.textContent =
      hardMet + "/" + hardOf + " hard · " + softMet + "/" + softOf + " soft";
  }

  function formatDateTime(iso) {
    if (!iso) return "";
    var d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    return (
      d.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      }) +
      " " +
      d.toLocaleTimeString(undefined, {
        hour: "numeric",
        minute: "2-digit",
      })
    );
  }

  function escapeHtml(s) {
    var div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }

  // --- State sync ---

  var syncTimer = null;

  function syncState(debounceMs) {
    if (isStatic) return;
    if (syncTimer) clearTimeout(syncTimer);

    var doSync = function () {
      var payload = structuredClone({
        requirements: state.requirements,
        items: state.items,
      });

      fetch("/state", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      })
        .then(function (resp) {
          if (!resp.ok) {
            return resp.json().then(function (body) {
              showToast("Sync failed: " + (body.error || "unknown error"));
              if (body.requirements && body.items) {
                state.requirements = body.requirements;
                state.items = body.items;
                render();
              }
            });
          }
        })
        .catch(function () {
          showToast("Sync failed: server unreachable");
          setTimeout(function () {
            syncState(0);
          }, 3000);
        });
    };

    if (debounceMs > 0) {
      syncTimer = setTimeout(doSync, debounceMs);
    } else {
      doSync();
    }
  }

  function syncFinalize(final) {
    if (isStatic) return;
    if (final) {
      fetch("/finalize", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(final),
      }).catch(function () {
        showToast("Failed to save decision");
      });
    } else {
      fetch("/finalize", { method: "DELETE" }).catch(function () {
        showToast("Failed to clear decision");
      });
    }
  }

  function showToast(msg) {
    var existing = document.querySelector(".toast");
    if (existing) existing.remove();
    var el = document.createElement("div");
    el.className = "toast";
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(function () {
      el.classList.add("visible");
    }, 10);
    setTimeout(function () {
      el.classList.remove("visible");
      setTimeout(function () {
        el.remove();
      }, 300);
    }, 3000);
  }

  // --- Load / Export ---

  var loadBtn = document.getElementById("load-btn");
  var loadInput = document.getElementById("load-file-input");
  if (loadBtn && loadInput) {
    loadBtn.addEventListener("click", function () {
      loadInput.click();
    });
    loadInput.addEventListener("change", function () {
      if (!loadInput.files.length) return;
      var form = new FormData();
      form.append("file", loadInput.files[0]);
      fetch("/upload", { method: "POST", body: form }).then(function (resp) {
        if (resp.ok) {
          window.location.reload();
        } else {
          resp.text().then(function (t) {
            alert("Load failed: " + t);
          });
        }
      });
    });
  }

  // --- Print / PDF ---

  var printBtn = document.getElementById("print-btn");
  if (printBtn) {
    printBtn.addEventListener("click", function () {
      window.print();
    });
  }

  // --- Export dropdown ---

  var exportBtn = document.getElementById("export-btn");
  var exportMenu = document.getElementById("export-menu");
  if (exportBtn && exportMenu) {
    exportBtn.addEventListener("click", function (e) {
      e.stopPropagation();
      var open = exportMenu.style.display !== "none";
      exportMenu.style.display = open ? "none" : "";
    });

    document.querySelectorAll(".export-option").forEach(function (opt) {
      opt.addEventListener("click", function (e) {
        e.stopPropagation();
        exportMenu.style.display = "none";
        window.location.href = "/export?format=" + this.dataset.format;
      });
    });

    document.addEventListener("click", function () {
      exportMenu.style.display = "none";
    });

    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && exportMenu.style.display !== "none") {
        exportMenu.style.display = "none";
        exportBtn.focus();
      }
    });
  }

  // Initial render
  render();
});
