import{t as e}from"./ordinal-hYBb2elL.js";import{t}from"./arc-LQh9717R.js";import{$t as n,An as r,Jt as i,Mt as a,P as o,Zt as s,_n as c,_t as l,en as u,hn as d,in as f,jn as p,jt as m,k as h,mn as g,mt as _,rn as v,tn as y}from"./index-B18mfE5Q.js";import{t as b}from"./mermaid-parser.core-BI7zVhP3.js";import{t as x}from"./chunk-4BX2VUAB-BD4auOvc.js";function S(e,t){return t<e?-1:t>e?1:t>=e?0:NaN}function C(e){return e}function w(){var e=C,t=S,n=null,r=a(0),i=a(m),o=a(0);function s(a){var s,c=(a=l(a)).length,u,d,f=0,p=Array(c),h=Array(c),g=+r.apply(this,arguments),_=Math.min(m,Math.max(-m,i.apply(this,arguments)-g)),v,y=Math.min(Math.abs(_)/c,o.apply(this,arguments)),b=y*(_<0?-1:1),x;for(s=0;s<c;++s)(x=h[p[s]=s]=+e(a[s],s,a))>0&&(f+=x);for(t==null?n!=null&&p.sort(function(e,t){return n(a[e],a[t])}):p.sort(function(e,n){return t(h[e],h[n])}),s=0,d=f?(_-c*b)/f:0;s<c;++s,g=v)u=p[s],x=h[u],v=g+(x>0?x*d:0)+b,h[u]={data:a[u],index:s,value:x,startAngle:g,endAngle:v,padAngle:y};return h}return s.value=function(t){return arguments.length?(e=typeof t==`function`?t:a(+t),s):e},s.sortValues=function(e){return arguments.length?(t=e,n=null,s):t},s.sort=function(e){return arguments.length?(n=e,t=null,s):n},s.startAngle=function(e){return arguments.length?(r=typeof e==`function`?e:a(+e),s):r},s.endAngle=function(e){return arguments.length?(i=typeof e==`function`?e:a(+e),s):i},s.padAngle=function(e){return arguments.length?(o=typeof e==`function`?e:a(+e),s):o},s}var T=n.pie,E={sections:new Map,showData:!1,config:T},D=E.sections,O=E.showData,k=structuredClone(T),A={getConfig:r(()=>structuredClone(k),`getConfig`),clear:r(()=>{D=new Map,O=E.showData,i()},`clear`),setDiagramTitle:c,getDiagramTitle:f,setAccTitle:d,getAccTitle:y,setAccDescription:g,getAccDescription:u,addSection:r(({label:e,value:t})=>{if(t<0)throw Error(`"${e}" has invalid value: ${t}. Negative values are not allowed in pie charts. All slice values must be >= 0.`);D.has(e)||(D.set(e,t),p.debug(`added new section: ${e}, with value: ${t}`))},`addSection`),getSections:r(()=>D,`getSections`),setShowData:r(e=>{O=e},`setShowData`),getShowData:r(()=>O,`getShowData`)},j=r((e,t)=>{x(e,t),t.setShowData(e.showData),e.sections.map(t.addSection)},`populateDb`),M={parse:r(async e=>{let t=await b(`pie`,e);p.debug(t),j(t,A)},`parse`)},N=r(e=>`
  .pieCircle{
    stroke: ${e.pieStrokeColor};
    stroke-width : ${e.pieStrokeWidth};
    opacity : ${e.pieOpacity};
  }
  .pieOuterCircle{
    stroke: ${e.pieOuterStrokeColor};
    stroke-width: ${e.pieOuterStrokeWidth};
    fill: none;
  }
  .pieTitleText {
    text-anchor: middle;
    font-size: ${e.pieTitleTextSize};
    fill: ${e.pieTitleTextColor};
    font-family: ${e.fontFamily};
  }
  .slice {
    font-family: ${e.fontFamily};
    fill: ${e.pieSectionTextColor};
    font-size:${e.pieSectionTextSize};
    // fill: white;
  }
  .legend text {
    fill: ${e.pieLegendTextColor};
    font-family: ${e.fontFamily};
    font-size: ${e.pieLegendTextSize};
  }
`,`getStyles`),P=r(e=>{let t=[...e.values()].reduce((e,t)=>e+t,0),n=[...e.entries()].map(([e,t])=>({label:e,value:t})).filter(e=>e.value/t*100>=1);return w().value(e=>e.value).sort(null)(n)},`createPieArcs`),F={parser:M,db:A,renderer:{draw:r((n,r,i,a)=>{p.debug(`rendering pie chart
`+n);let c=a.db,l=v(),u=h(c.getConfig(),l.pie),d=_(r),f=d.append(`g`);f.attr(`transform`,`translate(225,225)`);let{themeVariables:m}=l,[g]=o(m.pieOuterStrokeWidth);g??=2;let y=u.textPosition,b=t().innerRadius(0).outerRadius(185),x=t().innerRadius(185*y).outerRadius(185*y);f.append(`circle`).attr(`cx`,0).attr(`cy`,0).attr(`r`,185+g/2).attr(`class`,`pieOuterCircle`);let S=c.getSections(),C=P(S),w=[m.pie1,m.pie2,m.pie3,m.pie4,m.pie5,m.pie6,m.pie7,m.pie8,m.pie9,m.pie10,m.pie11,m.pie12],T=0;S.forEach(e=>{T+=e});let E=C.filter(e=>(e.data.value/T*100).toFixed(0)!==`0`),D=e(w).domain([...S.keys()]);f.selectAll(`mySlices`).data(E).enter().append(`path`).attr(`d`,b).attr(`fill`,e=>D(e.data.label)).attr(`class`,`pieCircle`),f.selectAll(`mySlices`).data(E).enter().append(`text`).text(e=>(e.data.value/T*100).toFixed(0)+`%`).attr(`transform`,e=>`translate(`+x.centroid(e)+`)`).style(`text-anchor`,`middle`).attr(`class`,`slice`);let O=f.append(`text`).text(c.getDiagramTitle()).attr(`x`,0).attr(`y`,-400/2).attr(`class`,`pieTitleText`),k=[...S.entries()].map(([e,t])=>({label:e,value:t})),A=f.selectAll(`.legend`).data(k).enter().append(`g`).attr(`class`,`legend`).attr(`transform`,(e,t)=>{let n=22*k.length/2;return`translate(216,`+(t*22-n)+`)`});A.append(`rect`).attr(`width`,18).attr(`height`,18).style(`fill`,e=>D(e.label)).style(`stroke`,e=>D(e.label)),A.append(`text`).attr(`x`,22).attr(`y`,14).text(e=>c.getShowData()?`${e.label} [${e.value}]`:e.label);let j=512+Math.max(...A.selectAll(`text`).nodes().map(e=>e?.getBoundingClientRect().width??0)),M=O.node()?.getBoundingClientRect().width??0,N=450/2-M/2,F=450/2+M/2,I=Math.min(0,N),L=Math.max(j,F)-I;d.attr(`viewBox`,`${I} 0 ${L} 450`),s(d,450,L,u.useMaxWidth)},`draw`)},styles:N};export{F as diagram};