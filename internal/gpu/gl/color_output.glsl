uniform sampler2D uImage;
uniform int uEncoding;
uniform float uWhite;
in vec2 uv;
out vec4 color;
vec3 linearize(vec3 c) {
    vec3 a=abs(c);
    return sign(c)*mix(pow((a+.055)/1.055,vec3(2.4)),a/12.92,lessThanEqual(a,vec3(.04045)));
}
vec3 srgb(vec3 c) {
    return mix(1.055*pow(c,vec3(1.0/2.4))-.055,12.92*c,lessThanEqual(c,vec3(.0031308)));
}
void main() {
    vec4 pixel=texture(uImage,uv);
    if (uEncoding==1) { color=pixel; return; }
    if (pixel.a<=0.0) { color=vec4(0.0); return; }
    vec3 rgb=linearize(pixel.rgb/pixel.a);
    if (uEncoding==2) {
        rgb=mat3(.82246197,.03319420,.01708263,.17753803,.96680580,.07239744,0.0,0.0,.91051993)*rgb;
        rgb=srgb(clamp(rgb,0.0,1.0));
    } else {
        rgb=mat3(.62740390,.06909729,.01639144,.32928304,.91954040,.08801331,.04331306,.01136231,.89559525)*rgb;
        vec3 p=pow(clamp(rgb*uWhite/10000.0,0.0,1.0),vec3(.1593017578125));
        rgb=pow((.8359375+18.8515625*p)/(1.0+18.6875*p),vec3(78.84375));
    }
    color=vec4(rgb*pixel.a,pixel.a);
}
