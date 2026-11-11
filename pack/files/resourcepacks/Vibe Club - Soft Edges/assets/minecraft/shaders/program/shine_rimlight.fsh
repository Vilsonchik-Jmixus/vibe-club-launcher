#version 150

uniform sampler2D DiffuseSampler;
uniform sampler2D DepthSampler;
uniform sampler2D RimMaskSampler;
uniform vec2 OutSize;
uniform float Thickness;
uniform float Strength;
uniform float DepthThreshold;
uniform float MaxDistance;
uniform float NearPlane;
uniform float FarPlane;

out vec4 fragColor;

const vec2 DIRECTIONS[4] = vec2[4](
    vec2(1.0, 0.0), vec2(0.0, 1.0),
    vec2(0.7071, 0.7071), vec2(-0.7071, 0.7071)
);

float viewDepth(vec2 uv) {
    float depth = texture(DepthSampler, uv).r;
    return NearPlane * FarPlane / max(FarPlane - depth * (FarPlane - NearPlane), 0.00001);
}

void main() {
    vec2 texel = 1.0 / OutSize;
    vec2 uv = gl_FragCoord.xy * texel;
    vec3 background = texture(DiffuseSampler, uv).rgb;
    float centerDepth = viewDepth(uv);
    vec2 radius = texel * clamp(Thickness * 0.5, 1.0, 2.0);
    vec2 lower = texel * 0.5;
    vec2 upper = 1.0 - lower;
    vec3 foreground = background;
    float closest = MaxDistance;
    float coverage = 0.0;

    // Extend the nearest masked surface across a depth break. The opposite
    // sample suppresses expansion on continuous sloped surfaces.
    for (int i = 0; i < 4; i++) {
        vec2 offset = DIRECTIONS[i] * radius;
        vec2 forward = clamp(uv + offset, lower, upper);
        vec2 backward = clamp(uv - offset, lower, upper);
        float frontDepth = viewDepth(forward);
        float backDepth = viewDepth(backward);
        vec2 source = frontDepth < backDepth ? forward : backward;
        float nearDepth = min(frontDepth, backDepth);
        float farDepth = max(frontDepth, backDepth);
        float threshold = max(nearDepth, 1.0) * max(DepthThreshold, 0.01) * 0.33;
        float depthBreak = centerDepth - nearDepth - max(farDepth - centerDepth, 0.0);
        if (depthBreak > threshold && nearDepth < closest && texture(RimMaskSampler, source).r > 0.5) {
            closest = nearDepth;
            foreground = texture(DiffuseSampler, source).rgb;
            coverage = smoothstep(threshold, threshold * 2.0, depthBreak);
        }
    }

    float distanceFade = 1.0 - smoothstep(MaxDistance * 0.65, MaxDistance, closest);
    float blend = coverage * distanceFade * clamp(Strength, 0.0, 1.0);
    fragColor = vec4(mix(background, foreground, blend), 1.0);
}
