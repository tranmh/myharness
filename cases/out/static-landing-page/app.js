document.getElementById('cta').addEventListener('click', function () {
  let msg = document.getElementById('cta-thank-you');
  if (!msg) {
    msg = document.createElement('p');
    msg.id = 'cta-thank-you';
    msg.textContent = 'Thanks for signing up! We\'ll be in touch soon.';
    msg.style.cssText = 'margin-top:1rem;color:#4f46e5;font-weight:600;font-size:1rem;';
    this.insertAdjacentElement('afterend', msg);
  } else {
    msg.style.display = msg.style.display === 'none' ? '' : 'none';
  }
});
