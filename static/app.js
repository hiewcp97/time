// App State
const state = {
  activeTab: 'search',
  searchQuery: '',
  selectedPlan: '',
  expiresBefore: '',
  
  // Keyset Pagination Stack (for Back/Next navigation)
  cursorHistory: [''], // Initial page has empty cursor
  currentCursorIndex: 0,
  nextCursor: '',

  // Active drawer customer
  selectedCustomer: null,

  // Jobs polling
  jobs: [],
  activePollingIntervals: {},

  // Job Items view
  activeJobID: '',
  jobItemsStatus: '',
  jobItemsLimit: 15,
  jobItemsOffset: 0
};

// Initialization
document.addEventListener('DOMContentLoaded', () => {
  // Set default date for contract expiration (next 30 days)
  const defaultDate = new Date();
  defaultDate.setDate(defaultDate.getDate() + 90); // 3 months out
  document.getElementById('expiry-filter').value = defaultDate.toISOString().split('T')[0];
  state.expiresBefore = document.getElementById('expiry-filter').value;
  
  // Load initial data
  loadCustomers();
  loadJobsList();
  
  // Poll jobs list status every 5 seconds
  setInterval(loadJobsList, 5000);
});

// Tab Switcher
function switchTab(tabId) {
  state.activeTab = tabId;
  
  // Update nav buttons
  document.getElementById('tab-search-btn').classList.toggle('active', tabId === 'search');
  document.getElementById('tab-jobs-btn').classList.toggle('active', tabId === 'jobs');
  
  // Update content panes
  document.getElementById('tab-search').classList.toggle('active', tabId === 'search');
  document.getElementById('tab-jobs').classList.toggle('active', tabId === 'jobs');
  
  if (tabId === 'jobs') {
    loadJobsList();
  }
}

// Fetch Customers List (with Cursor Pagination)
async function loadCustomers() {
  const tbody = document.getElementById('customers-tbody');
  tbody.innerHTML = `<tr><td colspan="5" style="text-align: center;"><i class="fa-solid fa-spinner fa-spin"></i> Loading customers...</td></tr>`;

  const cursor = state.cursorHistory[state.currentCursorIndex] || '';
  
  let url = `/api/v1/customers?limit=15&cursor=${encodeURIComponent(cursor)}`;
  if (state.searchQuery) url += `&search=${encodeURIComponent(state.searchQuery)}`;
  if (state.selectedPlan) url += `&plan=${encodeURIComponent(state.selectedPlan)}`;
  if (state.expiresBefore) url += `&expires_before=${encodeURIComponent(state.expiresBefore)}`;

  try {
    const res = await fetch(url);
    const result = await res.json();

    if (!result.data || result.data.length === 0) {
      tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted);">No expiring customers match these criteria.</td></tr>`;
      document.getElementById('pagination-info').innerText = 'Showing 0 customers';
      updatePaginationButtons();
      return;
    }

    state.nextCursor = result.next_cursor || '';
    
    tbody.innerHTML = '';
    result.data.forEach(cust => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>#${cust.id}</td>
        <td style="font-weight: 500;">${cust.name}</td>
        <td><span class="badge ${getPlanBadgeClass(cust.plan_name)}">${cust.plan_name}</span></td>
        <td><i class="fa-regular fa-calendar-days text-muted" style="margin-right: 8px;"></i>${cust.contract_end_date}</td>
        <td class="actions-col">
          <button class="btn btn-secondary btn-sm" onclick="openCustomerDrawer(${cust.id})">
            <i class="fa-regular fa-folder-open"></i> Open Files
          </button>
        </td>
      `;
      tbody.appendChild(tr);
    });

    const startIdx = state.currentCursorIndex * 15 + 1;
    const endIdx = startIdx + result.data.length - 1;
    document.getElementById('pagination-info').innerText = `Showing customer range ${startIdx}–${endIdx}`;
    
    updatePaginationButtons();

  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--danger);">Failed to load customers from backend. Ensure DB is seeded.</td></tr>`;
    console.error(err);
  }
}

// Keyset Pagination Controls
function prevPage() {
  if (state.currentCursorIndex > 0) {
    state.currentCursorIndex--;
    loadCustomers();
  }
}

function nextPage() {
  if (state.nextCursor) {
    state.currentCursorIndex++;
    state.cursorHistory[state.currentCursorIndex] = state.nextCursor;
    loadCustomers();
  }
}

function updatePaginationButtons() {
  document.getElementById('prev-btn').disabled = (state.currentCursorIndex === 0);
  document.getElementById('next-btn').disabled = (!state.nextCursor);
}

// Search Debouncer
let searchTimeout;
function debounceSearch() {
  clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => {
    state.searchQuery = document.getElementById('search-input').value.trim();
    resetPagination();
    loadCustomers();
  }, 350);
}

// Filter Actions
function applyFilters() {
  state.selectedPlan = document.getElementById('plan-filter').value;
  state.expiresBefore = document.getElementById('expiry-filter').value;
  resetPagination();
  loadCustomers();
}

function resetFilters() {
  document.getElementById('search-input').value = '';
  document.getElementById('plan-filter').value = '';
  
  const defaultDate = new Date();
  defaultDate.setDate(defaultDate.getDate() + 90);
  document.getElementById('expiry-filter').value = defaultDate.toISOString().split('T')[0];

  state.searchQuery = '';
  state.selectedPlan = '';
  state.expiresBefore = document.getElementById('expiry-filter').value;
  
  resetPagination();
  loadCustomers();
}

function resetPagination() {
  state.cursorHistory = [''];
  state.currentCursorIndex = 0;
  state.nextCursor = '';
}

// Open Customer Detail Drawer
async function openCustomerDrawer(id) {
  const overlay = document.getElementById('customer-drawer');
  overlay.classList.remove('hidden');

  // Reset Drawer State
  document.getElementById('drawer-customer-name').innerText = 'Loading details...';
  document.getElementById('drawer-customer-plan').innerText = '---';
  document.getElementById('drawer-cust-number').innerText = '---';
  document.getElementById('drawer-cust-fee').innerText = '---';
  document.getElementById('drawer-cust-tenure').innerText = '---';
  document.getElementById('drawer-cust-expiry').innerText = '---';
  document.getElementById('drawer-usage-container').innerHTML = '';
  
  resetPitchBox();

  try {
    const res = await fetch(`/api/v1/customers/${id}`);
    if (!res.ok) throw new Error();
    const details = await res.json();
    state.selectedCustomer = details;

    document.getElementById('drawer-customer-name').innerText = details.name;
    const planBadge = document.getElementById('drawer-customer-plan');
    planBadge.innerText = details.plan_name;
    planBadge.className = `badge ${getPlanBadgeClass(details.plan_name)}`;

    document.getElementById('drawer-cust-number').innerText = details.customer_number;
    document.getElementById('drawer-cust-fee').innerText = `$${details.monthly_fee.toFixed(2)}`;
    document.getElementById('drawer-cust-tenure').innerText = `${details.tenure_months} months`;
    document.getElementById('drawer-cust-expiry').innerText = details.contract_end_date;

    // Draw Usage History Visualizations
    const usageContainer = document.getElementById('drawer-usage-container');
    usageContainer.innerHTML = '';
    
    if (details.usage_history && details.usage_history.length > 0) {
      details.usage_history.forEach(use => {
        // Simple scaled bar chart relative to 1000GB download limit
        const downloadPct = Math.min((use.download_gb / 1000) * 100, 100);
        
        const row = document.createElement('div');
        row.className = 'usage-bar-row';
        row.innerHTML = `
          <span class="usage-month-label">${use.month}</span>
          <div class="usage-progress-container">
            <div class="usage-metric-bar">
              <div class="usage-metric-fill" style="width: ${downloadPct}%;"></div>
            </div>
            <div class="usage-val-text">
              <i class="fa-solid fa-cloud-arrow-down"></i> ${use.download_gb.toFixed(1)} GB &nbsp;&nbsp;&nbsp; 
              <i class="fa-solid fa-cloud-arrow-up"></i> ${use.upload_gb.toFixed(1)} GB
            </div>
          </div>
        `;
        usageContainer.appendChild(row);
      });
    } else {
      usageContainer.innerHTML = `<span style="font-size: 13px; color: var(--text-muted);">No usage records found.</span>`;
    }

    // Try loading existing cached pitch
    loadExistingPitch(id);

  } catch (err) {
    document.getElementById('drawer-customer-name').innerText = 'Error loading customer';
  }
}

function closeDrawer(event) {
  if (event === null || event.target === document.getElementById('customer-drawer')) {
    document.getElementById('customer-drawer').classList.add('hidden');
    state.selectedCustomer = null;
  }
}

// Load Existing Pitch from DB Cache
async function loadExistingPitch(customerID) {
  try {
    const res = await fetch(`/api/v1/customers/${customerID}/pitch`);
    if (res.ok) {
      const data = await res.json();
      showPitch(data.pitch, true);
    }
  } catch (err) {
    // If none exists, keep standard placeholder
  }
}

// Generate Pitch Request
async function generatePitch() {
  if (!state.selectedCustomer) return;
  
  const customerID = state.selectedCustomer.id;

  // Show spinner
  document.getElementById('pitch-placeholder').classList.add('hidden');
  document.getElementById('pitch-content-wrapper').classList.add('hidden');
  document.getElementById('pitch-spinner').classList.remove('hidden');

  try {
    const res = await fetch(`/api/v1/customers/${customerID}/pitch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.message || 'LLM generation failed');
    }
    
    const result = await res.json();
    showPitch(result.pitch, result.cached);

  } catch (err) {
    document.getElementById('pitch-spinner').classList.add('hidden');
    document.getElementById('pitch-placeholder').classList.remove('hidden');
    alert(`LLM Error: ${err.message}`);
  }
}

function showPitch(text, isCached) {
  document.getElementById('pitch-spinner').classList.add('hidden');
  document.getElementById('pitch-placeholder').classList.add('hidden');
  
  const contentWrapper = document.getElementById('pitch-content-wrapper');
  contentWrapper.classList.remove('hidden');
  document.getElementById('pitch-text').innerText = text;

  // Update Cached/Generated Badges
  document.getElementById('pitch-cache-badge').classList.toggle('hidden', !isCached);
  document.getElementById('pitch-fresh-badge').classList.toggle('hidden', isCached);
}

function resetPitchBox() {
  document.getElementById('pitch-placeholder').classList.remove('hidden');
  document.getElementById('pitch-content-wrapper').classList.add('hidden');
  document.getElementById('pitch-spinner').classList.add('hidden');
  document.getElementById('pitch-cache-badge').classList.add('hidden');
  document.getElementById('pitch-fresh-badge').classList.add('hidden');
}

function copyPitch() {
  const text = document.getElementById('pitch-text').innerText;
  navigator.clipboard.writeText(text).then(() => {
    const btn = document.querySelector('.btn-copy');
    btn.innerHTML = `<i class="fa-solid fa-check" style="color: var(--success);"></i> Copied!`;
    setTimeout(() => {
      btn.innerHTML = `<i class="fa-regular fa-copy"></i> Copy`;
    }, 1500);
  });
}

// Bulk Modal Actions
function openBulkModal() {
  document.getElementById('bulk-modal').classList.remove('hidden');
  document.getElementById('bulk-queue-error').classList.add('hidden');

  // Display active filters summary in modal
  document.getElementById('modal-filter-plan').innerText = state.selectedPlan || 'All Plans';
  document.getElementById('modal-filter-expiry').innerText = state.expiresBefore || 'Any Date';
}

function closeBulkModal(event) {
  if (event === null || event.target === document.getElementById('bulk-modal')) {
    document.getElementById('bulk-modal').classList.add('hidden');
  }
}

// Submit Bulk Job Request
async function submitBulkJob() {
  const reqBody = {
    filters: {
      expires_before: state.expiresBefore,
      plan_name: state.selectedPlan
    }
  };

  try {
    const res = await fetch('/api/v1/bulk-pitches', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(reqBody)
    });

    if (res.status === 429) {
      document.getElementById('bulk-queue-error').innerText = 'Bulk Job Queue is full (Max 1000). Please wait for ongoing jobs to complete.';
      document.getElementById('bulk-queue-error').classList.remove('hidden');
      return;
    }

    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.message || 'Failed to submit job');
    }

    const data = await res.json();
    document.getElementById('bulk-modal').classList.add('hidden');
    
    // Switch to status tracker and load
    switchTab('jobs');

  } catch (err) {
    document.getElementById('bulk-queue-error').innerText = `Error: ${err.message}`;
    document.getElementById('bulk-queue-error').classList.remove('hidden');
  }
}

// Load Bulk Jobs Statuses list
async function loadJobsList() {
  const container = document.getElementById('jobs-list-container');
  
  try {
    const res = await fetch('/api/v1/bulk-pitches');
    if (!res.ok) throw new Error();
    const result = await res.json();
    state.jobs = result.data || [];

    if (state.jobs.length === 0) {
      container.innerHTML = `
        <div class="card" style="grid-column: 1 / -1; padding: 40px; text-align: center; color: var(--text-muted); width: 100%;">
          <i class="fa-solid fa-list-check" style="font-size: 48px; margin-bottom: 16px; color: rgba(255,255,255,0.05); display: block;"></i>
          <p>No bulk jobs have been created yet. Launch one from the Customer Workspace!</p>
        </div>
      `;
      document.getElementById('active-jobs-badge').classList.add('hidden');
      return;
    }

    let hasActiveJobs = false;
    container.innerHTML = '';
    
    state.jobs.forEach(job => {
      const isProcessing = job.status === 'PENDING' || job.status === 'PROCESSING';
      if (isProcessing) {
        hasActiveJobs = true;
      }

      const total = job.total_count || 0;
      const completed = job.completed_count || 0;
      const failed = job.failed_count || 0;
      const processed = completed + failed;
      const progressPct = total > 0 ? Math.min((processed / total) * 100, 100) : 0;

      const card = document.createElement('div');
      card.className = `job-card card ${job.status.toLowerCase()}`;
      card.innerHTML = `
        <div class="job-card-header">
          <div>
            <div class="job-title">Job: ${job.job_id.substring(0, 8)}...</div>
            <div class="job-date"><i class="fa-regular fa-clock" style="margin-right: 6px;"></i>${formatDate(job.created_at)}</div>
          </div>
          <span class="badge ${getJobStatusBadgeClass(job.status)}">${job.status}</span>
        </div>

        <div class="progress-section">
          <div class="progress-bar-wrapper">
            <div class="progress-bar" style="width: ${progressPct}%;"></div>
          </div>
          <div class="progress-stats">
            <span>${progressPct.toFixed(0)}% (${processed}/${total})</span>
            <div>
              <span class="success-count">${completed} Ok</span> &nbsp;
              <span class="failed-count">${failed} Fail</span>
            </div>
          </div>
        </div>

        <div class="job-card-footer">
          <button class="btn btn-secondary btn-sm" onclick="openItemsModal('${job.job_id}')">
            <i class="fa-solid fa-list"></i> View Items
          </button>
        </div>
      `;
      container.appendChild(card);
    });

    document.getElementById('active-jobs-badge').classList.toggle('hidden', !hasActiveJobs);

  } catch (err) {
    container.innerHTML = `<div class="error-msg">Failed to load jobs list from backend server.</div>`;
    console.error(err);
  }
}

// Helpers
function getPlanBadgeClass(plan) {
  if (plan === '2Gbps') return 'badge-danger';
  if (plan === '1Gbps') return 'badge-purple';
  if (plan === '500Mbps') return 'badge-success';
  return 'badge-warning';
}

function getJobStatusBadgeClass(status) {
  if (status === 'COMPLETED') return 'badge-success';
  if (status === 'FAILED') return 'badge-danger';
  if (status === 'PROCESSING') return 'badge-warning';
  return 'badge-secondary';
}

function formatDate(isoStr) {
  if (!isoStr) return '---';
  const d = new Date(isoStr);
  return d.toLocaleString();
}

// Job Items Modal Actions
function openItemsModal(jobID) {
  state.activeJobID = jobID;
  state.jobItemsStatus = '';
  state.jobItemsOffset = 0;
  
  document.getElementById('items-modal-job-id').innerText = `${jobID.substring(0, 8)}...`;
  document.getElementById('items-modal').classList.remove('hidden');

  updateSubTabs();
  loadJobItems();
}

function closeItemsModal(event) {
  if (event === null || event.target === document.getElementById('items-modal')) {
    document.getElementById('items-modal').classList.add('hidden');
  }
}

function filterJobItems(status) {
  state.jobItemsStatus = status;
  state.jobItemsOffset = 0;
  updateSubTabs();
  loadJobItems();
}

function updateSubTabs() {
  document.getElementById('item-filter-all').classList.toggle('active', state.jobItemsStatus === '');
  document.getElementById('item-filter-success').classList.toggle('active', state.jobItemsStatus === 'SUCCESS');
  document.getElementById('item-filter-failed').classList.toggle('active', state.jobItemsStatus === 'FAILED');
}

async function loadJobItems() {
  const tbody = document.getElementById('job-items-tbody');
  tbody.innerHTML = `<tr><td colspan="3" style="text-align: center;"><i class="fa-solid fa-spinner fa-spin"></i> Loading job details...</td></tr>`;

  let url = `/api/v1/bulk-pitches/${state.activeJobID}/items?limit=${state.jobItemsLimit}&offset=${state.jobItemsOffset}`;
  if (state.jobItemsStatus) {
    url += `&status=${state.jobItemsStatus}`;
  }

  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error();
    const result = await res.json();
    const data = result.data || [];

    if (data.length === 0) {
      tbody.innerHTML = `<tr><td colspan="3" style="text-align: center; color: var(--text-muted);">No records matching filter.</td></tr>`;
      document.getElementById('item-next-btn').disabled = true;
      return;
    }

    tbody.innerHTML = '';
    data.forEach(item => {
      const tr = document.createElement('tr');
      const errorMsg = item.error_message ? item.error_message : '<span class="text-success">— Successful</span>';
      tr.innerHTML = `
        <td>Customer #${item.customer_id}</td>
        <td><span class="badge ${item.status === 'SUCCESS' ? 'badge-success' : 'badge-danger'}">${item.status}</span></td>
        <td style="font-family: var(--font-body); font-size: 13px;">${errorMsg}</td>
      `;
      tbody.appendChild(tr);
    });

    document.getElementById('item-prev-btn').disabled = (state.jobItemsOffset === 0);
    document.getElementById('item-next-btn').disabled = (data.length < state.jobItemsLimit);

  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="3" style="text-align: center; color: var(--danger);">Failed to load job items.</td></tr>`;
  }
}

function prevJobItems() {
  if (state.jobItemsOffset >= state.jobItemsLimit) {
    state.jobItemsOffset -= state.jobItemsLimit;
    loadJobItems();
  }
}

function nextJobItems() {
  state.jobItemsOffset += state.jobItemsLimit;
  loadJobItems();
}
