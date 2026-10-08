const figure=document.querySelector(".figure"),zoomButton=figure.querySelector(".zoom"),canvas=figure.querySelector(".figure__canvas");
const toggleZoom=()=>{const zoomed=figure.classList.toggle("is-zoomed");zoomButton.textContent=zoomed?"Zoom Out":"Zoom In";zoomButton.setAttribute("aria-pressed",zoomed);if(zoomed)canvas.scrollLeft=(canvas.scrollWidth-canvas.clientWidth)/2};
zoomButton.addEventListener("click",toggleZoom);
canvas.addEventListener("click",()=>{if(getComputedStyle(zoomButton).display!=="none"&&!figure.classList.contains("is-zoomed"))toggleZoom()});
