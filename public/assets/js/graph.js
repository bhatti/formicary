// Initialize Mermaid
mermaid.initialize({
    startOnLoad: false,
    theme: 'base',
    themeVariables: {
        primaryColor: '#e1f5fe',
        primaryTextColor: '#01579b',
        primaryBorderColor: '#01579b',
        lineColor: '#757575',
        secondaryColor: '#f5f5f5',
        tertiaryColor: '#ffffff'
    },
    flowchart: {
        htmlLabels: true,
        curve: 'linear'
    }
});

// Track last loaded URL to prevent duplicate loading
mermaid.lastLoadedUrl = null;

// Load and render Mermaid diagram
async function loadMermaidDiagram(url) {
    if (!url) {
        console.error('No URL provided for Mermaid diagram');
        return;
    }

    console.log('loadMermaidDiagram called with URL:', url);

    console.log('loadMermaidDiagram mermaid with URL:', url, ' mermaid: ', mermaid);
    console.log('loadMermaidDiagram comparing with URL:', url, ' against: ', mermaid.lastLoadedUrl);

    // Prevent loading the same diagram multiple times
    if (mermaid.lastLoadedUrl === url) {
        console.log('Diagram already loaded for URL:', url);
        return;
    }

    // Determine which diagram container to use
    let diagramDiv = document.getElementById('mermaid-diagram');
    let loadingDiv = document.getElementById('diagram-loading');
    let errorDiv = document.getElementById('mermaid-error');
    let errorMessage = document.getElementById('error-message');

    // If main containers don't exist, try definition containers
    if (!diagramDiv) {
        diagramDiv = document.getElementById('definition-mermaid-diagram');
        loadingDiv = document.getElementById('definition-diagram-loading');
        errorDiv = document.getElementById('definition-mermaid-error');
        errorMessage = document.getElementById('definition-error-message');
    }

    if (!diagramDiv) {
        console.error('No diagram container found');
        return;
    }

    try {
        // Show loading state
        if (loadingDiv) loadingDiv.style.display = 'block';
        if (errorDiv) errorDiv.classList.add('d-none');

        console.log('Fetching Mermaid diagram from:', url);
        const response = await fetch(url);

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const mermaidConfig = await response.text();
        console.log('Received Mermaid config:', mermaidConfig.substring(0, 100) + '...');

        if (!mermaidConfig.trim()) {
            throw new Error('Empty diagram configuration received');
        }

        // Generate unique ID for the diagram
        const timestamp = Date.now();
        const diagramId = 'job-graph-' + timestamp;

        // Clear the loading state
        if (loadingDiv) loadingDiv.style.display = 'none';

        // Render the diagram
        const { svg } = await mermaid.render(diagramId, mermaidConfig);
        diagramDiv.innerHTML = svg;

        // Mark this URL as loaded
        mermaid.lastLoadedUrl = url;
        console.log('Mermaid diagram rendered successfully for:', url);

    } catch (error) {
        console.error('Error loading Mermaid diagram:', error);
        if (loadingDiv) loadingDiv.style.display = 'none';
        if (errorDiv) errorDiv.classList.remove('d-none');
        if (errorMessage) errorMessage.textContent = error.message;
    }
}

// Clear last loaded URL (useful for navigation)
function clearMermaidCache() {
    mermaid.lastLoadedUrl = null;
}

// Export functions for job graphs
function exportJobGraphSVG() {
    const svg = document.querySelector('#mermaid-diagram svg') ||
        document.querySelector('#definition-mermaid-diagram svg');
    if (!svg) {
        alert('No diagram to export. Please load the diagram first.');
        return;
    }

    const svgData = new XMLSerializer().serializeToString(svg);
    const blob = new Blob([svgData], { type: 'image/svg+xml' });
    const url = URL.createObjectURL(blob);

    const a = document.createElement('a');
    a.href = url;
    a.download = getExportFilename('svg');
    a.click();

    URL.revokeObjectURL(url);
}


function exportJobGraphPNG() {
    const svg = document.querySelector('#mermaid-diagram svg') ||
        document.querySelector('#definition-mermaid-diagram svg');
    if (!svg) {
        alert('No diagram to export. Please load the diagram first.');
        return;
    }

    try {
        // Clone the SVG to avoid modifying the original
        const svgClone = svg.cloneNode(true);

        // Get the SVG dimensions
        const svgRect = svg.getBoundingClientRect();
        const width = svgRect.width || 800;  // fallback width
        const height = svgRect.height || 600; // fallback height

        // Set explicit dimensions on the clone
        svgClone.setAttribute('width', width);
        svgClone.setAttribute('height', height);

        // Add XML namespace if not present
        if (!svgClone.getAttribute('xmlns')) {
            svgClone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
        }

        // Include CSS styles inline
        const styles = getComputedStyle(svg);
        svgClone.style.cssText = styles.cssText;

        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        const img = new Image();

        // Set canvas dimensions
        canvas.width = width;
        canvas.height = height;

        // Error handler for image loading
        img.onerror = function(error) {
            console.error('Error loading SVG for PNG export:', error);
            alert('Failed to convert SVG to PNG. This might be due to browser security restrictions.');
        };

        img.onload = function () {
            try {
                // Fill with white background
                ctx.fillStyle = 'white';
                ctx.fillRect(0, 0, canvas.width, canvas.height);

                // Draw the SVG
                ctx.drawImage(img, 0, 0, width, height);

                // Convert to PNG and download
                canvas.toBlob(function (blob) {
                    if (blob) {
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        a.href = url;
                        a.download = getExportFilename('png');
                        document.body.appendChild(a); // Some browsers need this
                        a.click();
                        document.body.removeChild(a);
                        URL.revokeObjectURL(url);
                    } else {
                        alert('Failed to generate PNG blob');
                    }
                }, 'image/png', 1.0);

                URL.revokeObjectURL(img.src);
            } catch (error) {
                console.error('Error during canvas drawing:', error);
                alert('Failed to draw SVG on canvas');
            }
        };

        // Convert SVG to data URL
        const svgData = new XMLSerializer().serializeToString(svgClone);
        const svgDataUrl = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svgData);
        img.src = svgDataUrl;

    } catch (error) {
        console.error('Error during PNG export:', error);
        alert('Failed to export PNG: ' + error.message);
    }
}

// Alias functions for definition exports
function exportJobDefinitionSVG() {
    exportJobGraphSVG();
}

function exportJobDefinitionPNG() {
    exportJobGraphPNG();
}

// Helper function to get appropriate filename
function getExportFilename(extension) {
    const timestamp = new Date().toISOString().slice(0, 19).replace(/:/g, '-');

    // Try to get request ID first
    const requestIDElement = document.getElementById('requestID');
    if (requestIDElement && requestIDElement.innerText) {
        return `job-request-${requestIDElement.innerText}-graph.${extension}`;
    }

    // Try to get definition ID from URL or page context
    const urlParts = window.location.pathname.split('/');
    const possibleId = urlParts[urlParts.length - 1];
    if (possibleId && possibleId !== 'definitions' && possibleId !== 'requests') {
        return `job-definition-${possibleId}-graph.${extension}`;
    }

    // Fallback to timestamp
    return `job-graph-${timestamp}.${extension}`;
}