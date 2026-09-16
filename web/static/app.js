"use strict";

const LAST_SUBNET_KEY = "vectrixipam:lastSubnetId";

const state = {
  subnets: [],
  currentSubnetId: null,
};

const $ = (sel) => document.querySelector(sel);

function loadLastSubnetId() {
  try {
    const raw = localStorage.getItem(LAST_SUBNET_KEY);
    return raw ? Number(raw) : null;
  } catch {
    return null;
  }
}

function saveLastSubnetId(id) {
  try {
    if (id == null) {
      localStorage.removeItem(LAST_SUBNET_KEY);
    } else {
      localStorage.setItem(LAST_SUBNET_KEY, String(id));
    }
  } catch {
    // localStorage unavailable (private browsing etc.) — just skip persistence
  }
}

async function api(path, opts) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return null;
  return res.json();
}

// ---------- Subnets ----------

async function loadSubnets() {
  state.subnets = await api("/api/subnets");
  renderSubnets();
}

function renderSubnets() {
  const tbody = $("#subnets-table tbody");
  tbody.innerHTML = "";
  for (const s of state.subnets) {
    const tr = document.createElement("tr");
    tr.className = "clickable";
    tr.innerHTML = `
      <td>${escapeHTML(s.cidr)}</td>
      <td>${escapeHTML(s.description || "")}</td>
      <td>${escapeHTML(s.vlan || "")}</td>
      <td id="count-${s.id}">…</td>
      <td class="row-actions">
        <button class="secondary" data-open="${s.id}">Открыть</button>
        <button class="danger" data-delete-subnet="${s.id}">Удалить</button>
      </td>`;
    tbody.appendChild(tr);
    api(`/api/subnets/${s.id}/addresses`).then((addrs) => {
      const cell = document.getElementById(`count-${s.id}`);
      if (cell) cell.textContent = addrs ? addrs.length : 0;
    });
  }
}

$("#subnet-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const cidr = $("#subnet-cidr").value.trim();
  const description = $("#subnet-desc").value.trim();
  const vlan = $("#subnet-vlan").value.trim();
  try {
    await api("/api/subnets", { method: "POST", body: JSON.stringify({ cidr, description, vlan }) });
    e.target.reset();
    await loadSubnets();
  } catch (err) {
    alert("Не удалось добавить подсеть: " + err.message);
  }
});

$("#subnets-table").addEventListener("click", async (e) => {
  const openId = e.target.dataset.open;
  const deleteId = e.target.dataset.deleteSubnet;
  if (openId) {
    openSubnet(Number(openId));
  } else if (deleteId) {
    if (!confirm("Удалить подсеть вместе со всеми адресами?")) return;
    await api(`/api/subnets/${deleteId}`, { method: "DELETE" });
    if (state.currentSubnetId === Number(deleteId)) {
      state.currentSubnetId = null;
      saveLastSubnetId(null);
      $("#addresses-panel").hidden = true;
    }
    await loadSubnets();
  }
});

// ---------- Addresses ----------

async function openSubnet(id) {
  state.currentSubnetId = id;
  saveLastSubnetId(id);
  const subnet = state.subnets.find((s) => s.id === id);
  $("#current-subnet-label").textContent = subnet ? subnet.cidr : "";
  $("#addresses-panel").hidden = false;
  $("#discover-status").hidden = true;
  await loadAddresses();
}

async function loadAddresses() {
  if (!state.currentSubnetId) return;
  const addrs = await api(`/api/subnets/${state.currentSubnetId}/addresses`);
  renderAddresses(addrs || []);
}

function renderAddresses(addrs) {
  const tbody = $("#addresses-table tbody");
  tbody.innerHTML = "";
  for (const a of addrs) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${escapeHTML(a.ip)}</td>
      <td>${escapeHTML(a.hostname || "")}</td>
      <td>${escapeHTML(a.mac || "")}</td>
      <td><span class="badge ${a.status}">${escapeHTML(a.status)}</span></td>
      <td>${escapeHTML(a.owner || "")}</td>
      <td>${escapeHTML(a.notes || "")}</td>
      <td>${a.last_seen ? formatDate(a.last_seen) : "—"}</td>
      <td class="row-actions">
        <button class="secondary" data-edit="${a.id}">Изменить</button>
        <button class="danger" data-delete-addr="${a.id}">Удалить</button>
      </td>`;
    tbody.appendChild(tr);
  }
}

function formatDate(iso) {
  const d = new Date(iso);
  return isNaN(d) ? iso : d.toLocaleString();
}

$("#addresses-table").addEventListener("click", async (e) => {
  const editId = e.target.dataset.edit;
  const deleteId = e.target.dataset.deleteAddr;
  if (editId) {
    const addrs = await api(`/api/subnets/${state.currentSubnetId}/addresses`);
    const addr = addrs.find((a) => String(a.id) === editId);
    openAddressDialog(addr);
  } else if (deleteId) {
    if (!confirm("Удалить адрес?")) return;
    await api(`/api/addresses/${deleteId}`, { method: "DELETE" });
    await loadAddresses();
  }
});

$("#add-address-btn").addEventListener("click", () => openAddressDialog(null));

function openAddressDialog(addr) {
  $("#addr-id").value = addr ? addr.id : "";
  $("#addr-ip").value = addr ? addr.ip : "";
  $("#addr-ip").disabled = !!addr;
  $("#addr-hostname").value = addr ? addr.hostname : "";
  $("#addr-mac").value = addr ? addr.mac : "";
  $("#addr-status").value = addr ? addr.status : "used";
  $("#addr-owner").value = addr ? addr.owner : "";
  $("#addr-notes").value = addr ? addr.notes : "";
  $("#address-dialog").showModal();
}

$("#addr-cancel").addEventListener("click", () => $("#address-dialog").close());

$("#address-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const id = $("#addr-id").value;
  const payload = {
    ip: $("#addr-ip").value.trim(),
    hostname: $("#addr-hostname").value.trim(),
    mac: $("#addr-mac").value.trim(),
    status: $("#addr-status").value,
    owner: $("#addr-owner").value.trim(),
    notes: $("#addr-notes").value.trim(),
  };
  try {
    if (id) {
      await api(`/api/addresses/${id}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await api(`/api/subnets/${state.currentSubnetId}/addresses`, { method: "POST", body: JSON.stringify(payload) });
    }
    $("#address-dialog").close();
    await loadAddresses();
  } catch (err) {
    alert("Не удалось сохранить адрес: " + err.message);
  }
});

// ---------- Discovery ----------

$("#discover-btn").addEventListener("click", async () => {
  if (!state.currentSubnetId) return;
  const btn = $("#discover-btn");
  const status = $("#discover-status");
  btn.disabled = true;
  status.hidden = false;
  status.className = "status";
  status.textContent = "Сканирование подсети, это может занять до минуты…";
  try {
    const res = await api(`/api/subnets/${state.currentSubnetId}/discover`, { method: "POST" });
    const newCount = res.hosts.filter((h) => h.new).length;
    const macCount = res.hosts.filter((h) => h.mac).length;
    status.textContent = `Просканировано хостов: ${res.scanned}. Отвечающих: ${res.hosts.length} (новых: ${newCount}, с MAC: ${macCount}).`;
    await loadAddresses();
  } catch (err) {
    status.className = "status error";
    status.textContent = "Ошибка сканирования: " + err.message;
  } finally {
    btn.disabled = false;
  }
});

// ---------- Utils ----------

function escapeHTML(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

(async () => {
  try {
    await loadSubnets();
    const lastId = loadLastSubnetId();
    if (lastId != null && state.subnets.some((s) => s.id === lastId)) {
      await openSubnet(lastId);
    }
  } catch (err) {
    console.error(err);
  }
})();
