uniform highp samplerExternalOES uVideo;
uniform int uVideoRotation, uVideoFlip, uVideoTransfer, uVideoGamut;
uniform vec2 uVideoCrop, uVideoRange, uVideoSize;
uniform bool uVideoWide;
uniform float uVideoWhite, uVideoPeak;

vec3 videoLinearize(vec3 x) {
    if (!uVideoWide) x = clamp(x, 0.0, 1.0);
    if (uVideoTransfer == 1) {
        vec3 p = pow(clamp(x,0.0,1.0), vec3(1.0 / 78.84375));
        return 10000.0 * pow(max(p - .8359375, 0.0) / (18.8515625 - 18.6875 * p), vec3(1.0 / .1593017578125));
    }
    vec3 magnitude=abs(x);
    if (uVideoTransfer == 2)
        return sign(x)*mix((exp((magnitude - .55991073) / .17883277) + .28466892) / 12.0, magnitude*magnitude / 3.0, lessThanEqual(magnitude, vec3(.5)));
    if (uVideoTransfer == 4) return x;
    return sign(x)*mix(pow((magnitude+.055)/1.055, vec3(2.4)), magnitude/12.92, lessThanEqual(magnitude,vec3(.04045)));
}

vec4 videoTexel(vec2 position) {
    vec2 uv = (clamp(position, vec2(0.0), uVideoSize-1.0)+.5)/uVideoSize;
    if (uVideoRotation == 1) uv = vec2(uv.y,1.0-uv.x);
    else if (uVideoRotation == 2) uv = 1.0-uv;
    else if (uVideoRotation == 3) uv = vec2(1.0-uv.y,uv.x);
    if (uVideoFlip == 1) uv.x = 1.0-uv.x;
    else if (uVideoFlip == 2) uv.y = 1.0-uv.y;
    vec4 pixel = texelFetch(uVideo,ivec2(uv*uVideoCrop*vec2(textureSize(uVideo,0))),0);
    vec3 rgb = (pixel.rgb-uVideoRange.x)*uVideoRange.y;
    if (uVideoTransfer != 0) {
        rgb = videoLinearize(rgb);
        if (uVideoTransfer == 2) {
            float y = dot(rgb,uVideoGamut==2 ? vec3(.22897456,.69173852,.07928691) : uVideoGamut==1 ? vec3(.2627,.678,.0593) : vec3(.2126,.7152,.0722));
            float gamma=uVideoPeak>2000.0 ? 1.2*pow(1.111,log2(uVideoPeak/1000.0)) : 1.2+.42*log(uVideoPeak/1000.0)/log(10.0);
            rgb *= uVideoPeak*pow(max(y,0.0),gamma-1.0);
        }
        if (uVideoGamut == 2)
            rgb = mat3(1.22494018,-.04205695,-.01963755,-.22494018,1.04205695,-.07863605,0.0,0.0,1.09827360)*rgb;
        else if (uVideoGamut == 1)
            rgb = mat3(1.660491,-.124550,-.018151,-.587641,1.132900,-.100579,-.072850,-.008349,1.118730)*rgb;
        if (uVideoTransfer == 1 || uVideoTransfer == 2) {
            if (uVideoWide) rgb/=uVideoWhite;
            else {
                float y = max(dot(rgb,vec3(.2126,.7152,.0722)),0.0)/203.0;
                float w = 1000.0/203.0;
                rgb *= (1.0+y/(w*w))/((1.0+y)*203.0);
            }
        }
        if (!uVideoWide) rgb = clamp(rgb,0.0,1.0);
        vec3 magnitude=abs(rgb);
        rgb = sign(rgb)*mix(1.055*pow(magnitude,vec3(1.0/2.4))-.055,12.92*magnitude,lessThanEqual(magnitude,vec3(.0031308)));
    }
    return vec4((uVideoWide ? rgb : clamp(rgb,0.0,1.0))*pixel.a,pixel.a);
}

vec4 sampleVideo(vec2 uv, vec4 src, float pixelated, vec2 dtex) {
    // Match image sampling: convert and premultiply each texel before filtering.
    // Filtering straight alpha or encoded HDR first creates colored fringes and
    // differs from the on-demand RGBA conversion used by blur/shader inputs.
    if (pixelated > .5) {
        vec2 tc = uv*uVideoSize - .5;
        vec2 i = floor(tc);
        vec2 f = clamp((tc-i-.5)/clamp(dtex*uVideoSize,1e-5,1.0)+.5,0.0,1.0);
        uv = (i+f+.5)/uVideoSize;
    }
    uv = clamp(uv, min(src.xy,src.zw)+.5/uVideoSize, max(src.xy,src.zw)-.5/uVideoSize);
    vec2 tc = uv*uVideoSize-.5;
    vec2 p = floor(tc), f = tc-p;
    return mix(mix(videoTexel(p),videoTexel(p+vec2(1.0,0.0)),f.x),
               mix(videoTexel(p+vec2(0.0,1.0)),videoTexel(p+vec2(1.0)),f.x),f.y);
}
