using System;

namespace Moonwell
{
    /// <summary>
    /// A point or a direction on the map, as X and Y. A readonly struct: it never changes once made, so it is shared freely
    /// and never copied. Angles are in degrees, as Unit.Facing. Names follow Godot's Vector2 where the idea is the same.
    /// In the game a vector is a table to make; in code that runs every tick on many objects, use the float forms in
    /// <see cref="Maths"/>.
    /// </summary>
    public readonly struct Vector2 : IEquatable<Vector2>
    {
        /// <summary>The x coordinate.</summary>
        public readonly float X;

        /// <summary>The y coordinate.</summary>
        public readonly float Y;

        /// <summary>Makes a vector.</summary>
        [LocalSafe]
        public Vector2(float x, float y)
        {
            X = x;
            Y = y;
        }

        /// <summary>(0, 0).</summary>
        [LocalSafe]
        public static Vector2 Zero => new Vector2(0f, 0f);

        /// <summary>(1, 1).</summary>
        [LocalSafe]
        public static Vector2 One => new Vector2(1f, 1f);

        /// <summary>(1, 0): east on the map, facing 0.</summary>
        [LocalSafe]
        public static Vector2 Right => new Vector2(1f, 0f);

        /// <summary>(0, 1): north on the map, facing 90. Godot's Up points the other way, as its screen's y grows downward.</summary>
        [LocalSafe]
        public static Vector2 Up => new Vector2(0f, 1f);

        /// <summary>The vector's length.</summary>
        [LocalSafe]
        public float Length => Trig.Sqrt(X * X + Y * Y);

        /// <summary>The length squared: cheaper than Length, and enough to compare lengths.</summary>
        [LocalSafe]
        public float LengthSquared => X * X + Y * Y;

        /// <summary>The vector's direction in degrees, 0 to 360; 0 for a zero vector.</summary>
        [LocalSafe]
        public float Angle => Maths.AngleBetween(0f, 0f, X, Y);

        /// <summary>The vector with length 1 in the same direction; a zero vector gives zero.</summary>
        [LocalSafe]
        public Vector2 Normalized()
        {
            var length = Length;
            if (length == 0f)
            {
                return Zero;
            }

            return new Vector2(X / length, Y / length);
        }

        /// <summary>The distance from this point to another.</summary>
        [LocalSafe]
        public float DistanceTo(Vector2 other)
        {
            return Maths.Distance(X, Y, other.X, other.Y);
        }

        /// <summary>The distance to another point squared: cheaper than DistanceTo, and enough to compare distances.</summary>
        [LocalSafe]
        public float DistanceSquaredTo(Vector2 other)
        {
            return Maths.DistanceSquared(X, Y, other.X, other.Y);
        }

        /// <summary>The facing in degrees from this point to another, 0 to 360, as a unit's AngleTo; 0 for the same point. Godot's AngleTo is the angle between two directions instead.</summary>
        [LocalSafe]
        public float AngleTo(Vector2 point)
        {
            return Maths.AngleBetween(X, Y, point.X, point.Y);
        }

        /// <summary>The point a distance from this one in the direction of an angle in degrees.</summary>
        [LocalSafe]
        public Vector2 Offset(float degrees, float distance)
        {
            return new Vector2(X + distance * Angles.Cos(degrees), Y + distance * Angles.Sin(degrees));
        }

        /// <summary>The vector turned counterclockwise by an angle in degrees.</summary>
        [LocalSafe]
        public Vector2 Rotated(float degrees)
        {
            var cos = Angles.Cos(degrees);
            var sin = Angles.Sin(degrees);
            return new Vector2(X * cos - Y * sin, X * sin + Y * cos);
        }

        /// <summary>The dot product.</summary>
        [LocalSafe]
        public float Dot(Vector2 other)
        {
            return X * other.X + Y * other.Y;
        }

        /// <summary>The 2D cross product, a float: positive when other is counterclockwise from this vector.</summary>
        [LocalSafe]
        public float Cross(Vector2 other)
        {
            return X * other.Y - Y * other.X;
        }

        /// <summary>This point moved toward another by at most step, stopping on it.</summary>
        [LocalSafe]
        public Vector2 MoveToward(Vector2 to, float step)
        {
            var dx = to.X - X;
            var dy = to.Y - Y;
            var distance = Trig.Sqrt(dx * dx + dy * dy);
            if (distance <= step || distance == 0f)
            {
                return to;
            }

            var k = step / distance;
            return new Vector2(X + dx * k, Y + dy * k);
        }

        /// <summary>The vector shortened to max if it is longer; otherwise itself.</summary>
        [LocalSafe]
        public Vector2 LimitLength(float max)
        {
            var length = Length;
            if (length <= max || length == 0f)
            {
                return this;
            }

            var k = max / length;
            return new Vector2(X * k, Y * k);
        }

        /// <summary>The part of this vector along another; zero onto a zero vector.</summary>
        [LocalSafe]
        public Vector2 Project(Vector2 onto)
        {
            var lengthSquared = onto.LengthSquared;
            if (lengthSquared == 0f)
            {
                return Zero;
            }

            var k = Dot(onto) / lengthSquared;
            return new Vector2(onto.X * k, onto.Y * k);
        }

        /// <summary>The vector mirrored across a unit normal (Godot's reflect): (1, -1).Reflect(Up) is (-1, -1). For a bounce off a surface, use Bounce.</summary>
        [LocalSafe]
        public Vector2 Reflect(Vector2 normal)
        {
            var k = 2f * Dot(normal);
            return new Vector2(normal.X * k - X, normal.Y * k - Y);
        }

        /// <summary>The vector bounced off a surface with a unit normal: (1, -1).Bounce(Up) is (1, 1).</summary>
        [LocalSafe]
        public Vector2 Bounce(Vector2 normal)
        {
            var k = 2f * Dot(normal);
            return new Vector2(X - normal.X * k, Y - normal.Y * k);
        }

        /// <summary>The part of the vector along a surface with a unit normal, for sliding: (1, -1).Slide(Up) is (1, 0).</summary>
        [LocalSafe]
        public Vector2 Slide(Vector2 normal)
        {
            var k = Dot(normal);
            return new Vector2(X - normal.X * k, Y - normal.Y * k);
        }

        /// <summary>Whether each component is within tolerance of the other vector's. Use it on computed vectors, where == is exact.</summary>
        [LocalSafe]
        public bool Near(Vector2 other, float tolerance = 0.001f)
        {
            return Maths.Near(X, other.X, tolerance) && Maths.Near(Y, other.Y, tolerance);
        }

        /// <summary>The vector of a length in the direction of an angle in degrees.</summary>
        [LocalSafe]
        public static Vector2 FromAngle(float degrees, float length = 1f)
        {
            return new Vector2(length * Angles.Cos(degrees), length * Angles.Sin(degrees));
        }

        /// <summary>The point a fraction t of the way from a to b; t is not clamped.</summary>
        [LocalSafe]
        public static Vector2 Lerp(Vector2 a, Vector2 b, float t)
        {
            return new Vector2(a.X + (b.X - a.X) * t, a.Y + (b.Y - a.Y) * t);
        }

        /// <summary>The sum.</summary>
        [LocalSafe]
        public static Vector2 operator +(Vector2 a, Vector2 b)
        {
            return new Vector2(a.X + b.X, a.Y + b.Y);
        }

        /// <summary>The difference.</summary>
        [LocalSafe]
        public static Vector2 operator -(Vector2 a, Vector2 b)
        {
            return new Vector2(a.X - b.X, a.Y - b.Y);
        }

        /// <summary>The vector pointing the other way.</summary>
        [LocalSafe]
        public static Vector2 operator -(Vector2 v)
        {
            return new Vector2(-v.X, -v.Y);
        }

        /// <summary>The vector scaled.</summary>
        [LocalSafe]
        public static Vector2 operator *(Vector2 v, float k)
        {
            return new Vector2(v.X * k, v.Y * k);
        }

        /// <summary>The vector scaled.</summary>
        [LocalSafe]
        public static Vector2 operator *(float k, Vector2 v)
        {
            return new Vector2(v.X * k, v.Y * k);
        }

        /// <summary>The vector divided.</summary>
        [LocalSafe]
        public static Vector2 operator /(Vector2 v, float k)
        {
            return new Vector2(v.X / k, v.Y / k);
        }

        /// <summary>Whether both components are exactly equal.</summary>
        [LocalSafe]
        public static bool operator ==(Vector2 a, Vector2 b)
        {
            return a.X == b.X && a.Y == b.Y;
        }

        /// <summary>Whether a component differs.</summary>
        [LocalSafe]
        public static bool operator !=(Vector2 a, Vector2 b)
        {
            return a.X != b.X || a.Y != b.Y;
        }

        /// <summary>Whether both components are exactly equal.</summary>
        [LocalSafe]
        public bool Equals(Vector2 other)
        {
            return X == other.X && Y == other.Y;
        }

        /// <summary>Whether the object is a Vector2 with both components exactly equal.</summary>
        [LocalSafe]
        public override bool Equals(object? obj)
        {
            return obj is Vector2 other && Equals(other);
        }

        /// <summary>A hash of both components.</summary>
        [LocalSafe]
        public override int GetHashCode()
        {
            return X.GetHashCode() ^ (Y.GetHashCode() * 397);
        }

        /// <summary>The vector as text: (12.5, 40).</summary>
        [LocalSafe]
        public override string ToString()
        {
            return "(" + X + ", " + Y + ")";
        }
    }
}
