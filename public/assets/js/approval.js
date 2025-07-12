/**
 * The handler function for the form submission.
 * This function is called by the onsubmit event.
 */
function handleReviewSubmit(form) {
    // Get the clicked button (the one that triggered the submit)
    const activeElement = document.activeElement;
    let status = null;

    // Check if the active element is one of our submit buttons
    if (activeElement && activeElement.name === 'statusB') {
        status = activeElement.value;
    } else {
        // Fallback: try to find which button was clicked by checking form data
        const formData = new FormData(form);
        status = formData.get('statusB');
    }

    // Validate that we have a status
    if (!status || (status !== 'APPROVED' && status !== 'REJECTED')) {
        alert('Please select either Approve or Reject.');
        return false; // Prevent form submission
    }

    // Get comments using form elements
    const commentsInput = form.elements['comments'];
    const comments = commentsInput ? commentsInput.value.trim() : '';

    // Validation - comments required for rejection
    if (status === 'REJECTED' && !comments) {
        alert('Comments are required when rejecting.');
        if (commentsInput) commentsInput.focus();
        return false; // Prevent form submission
    }

    // Confirmation
    const isTaskForm = form.id === 'taskReviewForm';
    const itemType = isTaskForm ? 'task' : 'job';
    const action = status === 'APPROVED' ? 'approve' : 'reject';

    console.log('The submit handler was called for form:', form.id, 'status: ', status, 'comments: ', comments);
    if (confirm(`Are you sure you want to ${action} this ${itemType}?`)) {
        console.log(`User confirmed. Submitting form with status: ${status}`);

        // Add the status as a hidden input to ensure it gets submitted
        // (in case the button value doesn't get sent properly)
        let hiddenStatusInput = form.elements['status'];
        if (!hiddenStatusInput || hiddenStatusInput.type !== 'hidden') {
            // Create hidden input if it doesn't exist
            hiddenStatusInput = document.createElement('input');
            hiddenStatusInput.type = 'hidden';
            hiddenStatusInput.name = 'status';
            form.appendChild(hiddenStatusInput);
        }
        hiddenStatusInput.value = status;

        return true; // Allow form submission
    } else {
        console.log("User cancelled submission.");
        return false; // Prevent form submission
    }
}
