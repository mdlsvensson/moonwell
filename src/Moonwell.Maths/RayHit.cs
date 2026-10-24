namespace Moonwell
{
    /// <summary>Where a raycast hit: the point, the surface's normal there, and how far along the segment.</summary>
    public readonly struct RayHit
    {
        /// <summary>The point hit.</summary>
        public readonly Vector2 Point;

        /// <summary>The surface's normal at the point, of length 1, facing back toward the segment's start: what Bounce and Slide take. Zero for a zero-length segment that starts inside.</summary>
        public readonly Vector2 Normal;

        /// <summary>How far along the segment: 0 at its start, 1 at its end.</summary>
        public readonly float Fraction;

        /// <summary>Makes a hit.</summary>
        [LocalSafe]
        public RayHit(Vector2 point, Vector2 normal, float fraction)
        {
            Point = point;
            Normal = normal;
            Fraction = fraction;
        }
    }
}
