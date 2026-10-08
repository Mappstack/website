const SignupEndpoint="https://stackure.com/api/public/signup",MappstackAccountID="3df5678e-9c26-4384-9c51-7ae628b7f46a";
const signupForm=document.querySelector(".signup"),signupStatus=signupForm.querySelector(".signup__status"),sendButton=signupForm.querySelector("[type=submit]");
const showSignupStatus=message=>{signupStatus.textContent=message;signupStatus.hidden=false;sendButton.disabled=false};
const submitSignup=async event=>{
  event.preventDefault();
  sendButton.disabled=true;
  signupStatus.hidden=true;
  try{
    const response=await fetch(SignupEndpoint,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({...Object.fromEntries(new FormData(signupForm)),parent_id:MappstackAccountID,account_type:"Customer",account_plan:"pilot"})});
    if(response.ok)return signupForm.replaceWith(Object.assign(document.createElement("p"),{className:"lead",textContent:"Got it. We'll be in touch soon."}));
    showSignupStatus(response.status===429?"Too many attempts. Please try again later.":"Didn't work. Give it a moment.");
  }catch{showSignupStatus("Didn't work. Give it a moment.")}
};
signupForm.addEventListener("submit",submitSignup);
