using System;

namespace Moonwell
{
    /// <summary>Where a ray met the ground: the point, the ground's normal there, and how far along the segment.</summary>
    public readonly struct TerrainHit : IEquatable<TerrainHit>
    {
        /// <summary>The point hit.</summary>
        public readonly Vector3 Point;

        /// <summary>The ground's normal at the point, of length 1 (<c>Terrain.Normal</c> there): straight up when the segment starts under the ground (by more than 0.02).</summary>
        public readonly Vector3 Normal;

        /// <summary>How far along the segment: 0 at its start, 1 at its end.</summary>
        public readonly float Fraction;

        /// <summary>Makes a hit.</summary>
        [LocalSafe]
        public TerrainHit(Vector3 point, Vector3 normal, float fraction)
        {
            Point = point;
            Normal = normal;
            Fraction = fraction;
        }

        /// <summary>Whether the point, normal and fraction are exactly equal.</summary>
        [LocalSafe]
        public static bool operator ==(TerrainHit a, TerrainHit b)
        {
            return a.Point == b.Point && a.Normal == b.Normal && a.Fraction == b.Fraction;
        }

        /// <summary>Whether the point, normal or fraction differs.</summary>
        [LocalSafe]
        public static bool operator !=(TerrainHit a, TerrainHit b)
        {
            return a.Point != b.Point || a.Normal != b.Normal || a.Fraction != b.Fraction;
        }

        /// <summary>Whether the point, normal and fraction are exactly equal.</summary>
        [LocalSafe]
        public bool Equals(TerrainHit other)
        {
            return Point == other.Point && Normal == other.Normal && Fraction == other.Fraction;
        }

        /// <summary>Whether the object is a TerrainHit with the same point, normal and fraction.</summary>
        [LocalSafe]
        public override bool Equals(object? obj)
        {
            return obj is TerrainHit other && Equals(other);
        }

        /// <summary>A hash of the point, normal and fraction.</summary>
        [LocalSafe]
        public override int GetHashCode()
        {
            return Point.GetHashCode() ^ (Normal.GetHashCode() * 397) ^ Fraction.GetHashCode();
        }

        /// <summary>The hit as text: ((0, 0, 5), (0, 0, 1), 0.5).</summary>
        [LocalSafe]
        public override string ToString()
        {
            return "(" + Point + ", " + Normal + ", " + Fraction + ")";
        }
    }
}
