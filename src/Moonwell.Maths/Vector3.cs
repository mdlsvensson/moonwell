using System;

namespace Moonwell
{
    /// <summary>
    /// A point or a direction in the world, as X, Y and Z, where Z is height. A readonly struct, as <see cref="Vector2"/>:
    /// it never changes once made, so it is shared freely and never copied. Angles are in degrees. In code that runs every
    /// tick on many objects, keep floats instead; a vector is a table to make.
    /// </summary>
    public readonly struct Vector3 : IEquatable<Vector3>
    {
        /// <summary>The x coordinate.</summary>
        public readonly float X;

        /// <summary>The y coordinate.</summary>
        public readonly float Y;

        /// <summary>The z coordinate: height.</summary>
        public readonly float Z;

        /// <summary>Makes a vector.</summary>
        [LocalSafe]
        public Vector3(float x, float y, float z)
        {
            X = x;
            Y = y;
            Z = z;
        }

        /// <summary>Makes a vector from a point on the map and a height.</summary>
        [LocalSafe]
        public Vector3(Vector2 xy, float z)
        {
            X = xy.X;
            Y = xy.Y;
            Z = z;
        }

        /// <summary>(0, 0, 0).</summary>
        [LocalSafe]
        public static Vector3 Zero => new Vector3(0f, 0f, 0f);

        /// <summary>(1, 1, 1).</summary>
        [LocalSafe]
        public static Vector3 One => new Vector3(1f, 1f, 1f);

        /// <summary>(1, 0, 0): east.</summary>
        [LocalSafe]
        public static Vector3 UnitX => new Vector3(1f, 0f, 0f);

        /// <summary>(0, 1, 0): north.</summary>
        [LocalSafe]
        public static Vector3 UnitY => new Vector3(0f, 1f, 0f);

        /// <summary>(0, 0, 1): straight up.</summary>
        [LocalSafe]
        public static Vector3 UnitZ => new Vector3(0f, 0f, 1f);

        /// <summary>The vector's length.</summary>
        [LocalSafe]
        public float Length => Trig.Sqrt(X * X + Y * Y + Z * Z);

        /// <summary>The length squared: cheaper than Length, and enough to compare lengths.</summary>
        [LocalSafe]
        public float LengthSquared => X * X + Y * Y + Z * Z;

        /// <summary>The point on the map, without the height.</summary>
        [LocalSafe]
        public Vector2 XY => new Vector2(X, Y);

        /// <summary>The direction on the map in degrees, 0 to 360, as a facing: what an effect's yaw needs to point along the vector. 0 straight up or down.</summary>
        [LocalSafe]
        public float Yaw => Maths.AngleBetween(0f, 0f, X, Y);

        /// <summary>The tilt up from the map in degrees, -90 to 90: 90 straight up. What an effect's pitch needs to point along the vector. 0 for a zero vector.</summary>
        [LocalSafe]
        public float Pitch
        {
            get
            {
                if (X == 0f && Y == 0f && Z == 0f)
                {
                    return 0f;
                }

                return Angles.ToDegrees(Trig.Atan2(Z, Trig.Sqrt(X * X + Y * Y)));
            }
        }

        /// <summary>The vector with length 1 in the same direction; a zero vector gives zero.</summary>
        [LocalSafe]
        public Vector3 Normalized()
        {
            var length = Length;
            if (length == 0f)
            {
                return Zero;
            }

            return new Vector3(X / length, Y / length, Z / length);
        }

        /// <summary>The distance from this point to another.</summary>
        [LocalSafe]
        public float DistanceTo(Vector3 other)
        {
            return Trig.Sqrt(DistanceSquaredTo(other));
        }

        /// <summary>The distance to another point squared: cheaper than DistanceTo, and enough to compare distances.</summary>
        [LocalSafe]
        public float DistanceSquaredTo(Vector3 other)
        {
            var dx = other.X - X;
            var dy = other.Y - Y;
            var dz = other.Z - Z;
            return dx * dx + dy * dy + dz * dz;
        }

        /// <summary>The dot product.</summary>
        [LocalSafe]
        public float Dot(Vector3 other)
        {
            return X * other.X + Y * other.Y + Z * other.Z;
        }

        /// <summary>The cross product, by the right hand: UnitX.Cross(UnitY) is UnitZ.</summary>
        [LocalSafe]
        public Vector3 Cross(Vector3 other)
        {
            return new Vector3(Y * other.Z - Z * other.Y, Z * other.X - X * other.Z, X * other.Y - Y * other.X);
        }

        /// <summary>This point moved toward another by at most step, stopping on it.</summary>
        [LocalSafe]
        public Vector3 MoveToward(Vector3 to, float step)
        {
            var dx = to.X - X;
            var dy = to.Y - Y;
            var dz = to.Z - Z;
            var distance = Trig.Sqrt(dx * dx + dy * dy + dz * dz);
            if (distance <= step || distance == 0f)
            {
                return to;
            }

            var k = step / distance;
            return new Vector3(X + dx * k, Y + dy * k, Z + dz * k);
        }

        /// <summary>The vector shortened to max if it is longer; otherwise itself.</summary>
        [LocalSafe]
        public Vector3 LimitLength(float max)
        {
            var length = Length;
            if (length <= max || length == 0f)
            {
                return this;
            }

            var k = max / length;
            return new Vector3(X * k, Y * k, Z * k);
        }

        /// <summary>The part of this vector along another; zero onto a zero vector.</summary>
        [LocalSafe]
        public Vector3 Project(Vector3 onto)
        {
            var lengthSquared = onto.LengthSquared;
            if (lengthSquared == 0f)
            {
                return Zero;
            }

            var k = Dot(onto) / lengthSquared;
            return new Vector3(onto.X * k, onto.Y * k, onto.Z * k);
        }

        /// <summary>The vector mirrored across a unit normal (Godot's reflect). For a bounce off a surface, use Bounce.</summary>
        [LocalSafe]
        public Vector3 Reflect(Vector3 normal)
        {
            var k = 2f * Dot(normal);
            return new Vector3(normal.X * k - X, normal.Y * k - Y, normal.Z * k - Z);
        }

        /// <summary>The vector bounced off a surface with a unit normal: (1, 0, -1).Bounce(UnitZ) is (1, 0, 1).</summary>
        [LocalSafe]
        public Vector3 Bounce(Vector3 normal)
        {
            var k = 2f * Dot(normal);
            return new Vector3(X - normal.X * k, Y - normal.Y * k, Z - normal.Z * k);
        }

        /// <summary>The part of the vector along a surface with a unit normal, for sliding.</summary>
        [LocalSafe]
        public Vector3 Slide(Vector3 normal)
        {
            var k = Dot(normal);
            return new Vector3(X - normal.X * k, Y - normal.Y * k, Z - normal.Z * k);
        }

        /// <summary>Whether each component is within tolerance of the other vector's. Use it on computed vectors, where == is exact.</summary>
        [LocalSafe]
        public bool Near(Vector3 other, float tolerance = 0.001f)
        {
            return Maths.Near(X, other.X, tolerance) && Maths.Near(Y, other.Y, tolerance) && Maths.Near(Z, other.Z, tolerance);
        }

        /// <summary>The vector of a length pointing along a yaw (the direction on the map) and a pitch (the tilt up), in degrees.</summary>
        [LocalSafe]
        public static Vector3 FromAngles(float yaw, float pitch, float length = 1f)
        {
            var flat = length * Angles.Cos(pitch);
            return new Vector3(flat * Angles.Cos(yaw), flat * Angles.Sin(yaw), length * Angles.Sin(pitch));
        }

        /// <summary>The point a fraction t of the way from a to b; t is not clamped.</summary>
        [LocalSafe]
        public static Vector3 Lerp(Vector3 a, Vector3 b, float t)
        {
            return new Vector3(a.X + (b.X - a.X) * t, a.Y + (b.Y - a.Y) * t, a.Z + (b.Z - a.Z) * t);
        }

        /// <summary>The sum.</summary>
        [LocalSafe]
        public static Vector3 operator +(Vector3 a, Vector3 b)
        {
            return new Vector3(a.X + b.X, a.Y + b.Y, a.Z + b.Z);
        }

        /// <summary>The difference.</summary>
        [LocalSafe]
        public static Vector3 operator -(Vector3 a, Vector3 b)
        {
            return new Vector3(a.X - b.X, a.Y - b.Y, a.Z - b.Z);
        }

        /// <summary>The vector pointing the other way.</summary>
        [LocalSafe]
        public static Vector3 operator -(Vector3 v)
        {
            return new Vector3(-v.X, -v.Y, -v.Z);
        }

        /// <summary>The vector scaled.</summary>
        [LocalSafe]
        public static Vector3 operator *(Vector3 v, float k)
        {
            return new Vector3(v.X * k, v.Y * k, v.Z * k);
        }

        /// <summary>The vector scaled.</summary>
        [LocalSafe]
        public static Vector3 operator *(float k, Vector3 v)
        {
            return new Vector3(v.X * k, v.Y * k, v.Z * k);
        }

        /// <summary>The vector divided.</summary>
        [LocalSafe]
        public static Vector3 operator /(Vector3 v, float k)
        {
            return new Vector3(v.X / k, v.Y / k, v.Z / k);
        }

        /// <summary>Whether all components are exactly equal.</summary>
        [LocalSafe]
        public static bool operator ==(Vector3 a, Vector3 b)
        {
            return a.X == b.X && a.Y == b.Y && a.Z == b.Z;
        }

        /// <summary>Whether a component differs.</summary>
        [LocalSafe]
        public static bool operator !=(Vector3 a, Vector3 b)
        {
            return a.X != b.X || a.Y != b.Y || a.Z != b.Z;
        }

        /// <summary>Whether all components are exactly equal.</summary>
        [LocalSafe]
        public bool Equals(Vector3 other)
        {
            return X == other.X && Y == other.Y && Z == other.Z;
        }

        /// <summary>Whether the object is a Vector3 with all components exactly equal.</summary>
        [LocalSafe]
        public override bool Equals(object? obj)
        {
            return obj is Vector3 other && Equals(other);
        }

        /// <summary>A hash of the components.</summary>
        [LocalSafe]
        public override int GetHashCode()
        {
            return X.GetHashCode() ^ (Y.GetHashCode() * 397) ^ (Z.GetHashCode() * 7919);
        }

        /// <summary>The vector as text: (1, 2.5, 0).</summary>
        [LocalSafe]
        public override string ToString()
        {
            return "(" + X + ", " + Y + ", " + Z + ")";
        }
    }
}
