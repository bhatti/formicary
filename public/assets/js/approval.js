/**
 * Approval vote form submit handler.
 * Validates that a decision was selected and comments are present for rejections.
 */
function handleReviewSubmit(form) {
    const activeElement = document.activeElement;
    let decision = null;

    if (activeElement && activeElement.name === 'decision') {
        decision = activeElement.value;
    } else {
        const formData = new FormData(form);
        decision = formData.get('decision');
    }

    if (!decision || (decision !== 'APPROVED' && decision !== 'REJECTED')) {
        alert('Please select either Approve or Reject.');
        return false;
    }

    const commentsInput = form.elements['comments'];
    const comments = commentsInput ? commentsInput.value.trim() : '';

    if (decision === 'REJECTED' && !comments) {
        alert('Comments are required when rejecting.');
        if (commentsInput) commentsInput.focus();
        return false;
    }

    const action = decision === 'APPROVED' ? 'approve' : 'reject';
    return confirm(`Are you sure you want to ${action} this task?`);
}
