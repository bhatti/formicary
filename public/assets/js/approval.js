// Enhanced approval modal with validation
function validateApprovalForm(formId) {
    const form = document.getElementById(formId);
    const taskType = form.querySelector('input[name="taskType"]').value;

    if (!taskType || taskType.trim() === '') {
        alert('Error: Task type is required for approval');
        return false;
    }

    return confirm('Are you sure you want to approve this ' +
                  (formId.includes('Task') ? 'task' : 'job') + '?');
}

// Auto-refresh for pending approvals page
function setupApprovalPageRefresh() {
    if (window.location.pathname.includes('pending-approvals')) {
        setInterval(function() {
            window.location.reload();
        }, 30000); // Refresh every 30 seconds
    }
}

// Initialize on page load
$(document).ready(function() {
    setupApprovalPageRefresh();

    // Add validation to approval forms
    $('form[action*="/approve"]').on('submit', function(e) {
        const formId = this.id || 'approvalForm';
        if (!validateApprovalForm(formId)) {
            e.preventDefault();
            return false;
        }
    });
});
