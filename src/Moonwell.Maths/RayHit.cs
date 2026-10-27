using System;

namespace Moonwell
{
    /// <summary>Where a raycast hit: the point, the surface's normal there, and how far along the segment.</summary>
    public readonly struct RayHit : IEquatable<RayHit>
    {
        /// <summary>The point hit.</summary>
        public readonly Vector2 Point;

        /// <summary>The surface's normal at the point, of length 1, facing back toward the segment's start: what Bounce and Slide take. Zero for a zero-length segment that starts inside (deeper than 0.02).</summary>
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

        /// <summary>Whether the point, normal and fraction are exactly equal.</summary>
        [LocalSafe]
        public static bool operator ==(RayHit a, RayHit b)
        {
            return a.Point == b.Point && a.Normal == b.Normal && a.Fraction == b.Fraction;
        }

        /// <summary>Whether the point, normal or fraction differs.</summary>
        [LocalSafe]
        public static bool operator !=(RayHit a, RayHit b)
        {
            return a.Point != b.Point || a.Normal != b.Normal || a.Fraction != b.Fraction;
        }

        /// <summary>Whether the point, normal and fraction are exactly equal.</summary>
        [LocalSafe]
        public bool Equals(RayHit other)
        {
            return Point == other.Point && Normal == other.Normal && Fraction == other.Fraction;
        }

        /// <summary>Whether the object is a RayHit with the same point, normal and fraction.</summary>
        [LocalSafe]
        public override bool Equals(object? obj)
        {
            return obj is RayHit other && Equals(other);
        }

        /// <summary>A hash of the point, normal and fraction.</summary>
        [LocalSafe]
        public override int GetHashCode()
        {
            return Point.GetHashCode() ^ (Normal.GetHashCode() * 397) ^ Fraction.GetHashCode();
        }

        /// <summary>The hit as text: ((0, 5), (-1, 0), 0.5).</summary>
        [LocalSafe]
        public override string ToString()
        {
            return "(" + Point + ", " + Normal + ", " + Fraction + ")";
        }
    }
}
