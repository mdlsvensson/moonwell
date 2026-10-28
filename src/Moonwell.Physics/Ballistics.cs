namespace Moonwell
{
    /// <summary>
    /// Launches for a thrown thing under gravity, in closed forms: the velocity to hit a point at a given speed, and the
    /// speed to hit it at a given angle. A thing launched with the velocity, under the same gravity, passes through the
    /// point. Nothing here changes anything in the game, so all of it is safe inside Local.For.
    /// </summary>
    public static class Ballistics
    {
        /// <summary>The horizontal distance below which a shot counts as straight up or down.</summary>
        private const float Vertical = 0.0001f;

        /// <summary>The cosine of the launch angle below which a launch counts as straight up or down, which no speed carries along the ground.</summary>
        private const float Steep = 0.0001f;

        /// <summary>
        /// The velocity that carries a thing from a point to another at a speed, under a gravity, or false when the point is
        /// out of reach (the velocity is then zero). There are two arcs: the low one, and the high one when <paramref name="high"/>
        /// is true; they meet when the point is at the edge of what the speed reaches. With no gravity the launch is
        /// straight at the point and <paramref name="high"/> changes nothing. A point straight above or below is reached
        /// by a vertical launch, within the height the speed climbs to. False too for a speed of 0 or less, and for a point
        /// that is the start when there is no gravity.
        /// </summary>
        [LocalSafe]
        public static bool Velocity(Vector3 from, Vector3 to, float speed, bool high, float gravity, out Vector3 velocity)
        {
            velocity = Vector3.Zero;
            if (speed <= 0.0f)
            {
                return false;
            }

            var dx = to.X - from.X;
            var dy = to.Y - from.Y;
            var dz = to.Z - from.Z;
            if (gravity == 0.0f)
            {
                var length = Trig.Sqrt(dx * dx + dy * dy + dz * dz);
                if (length == 0.0f)
                {
                    return false;
                }

                var scale = speed / length;
                velocity = new Vector3(dx * scale, dy * scale, dz * scale);
                return true;
            }

            var distance = Trig.Sqrt(dx * dx + dy * dy);
            if (distance < Vertical)
            {
                return Straight(dz, speed, gravity, out velocity);
            }

            // With t the tangent of the launch angle, the height at the point is
            // dz = d t - (g d^2 / (2 v^2)) (1 + t^2), which is a t^2 - d t + (dz + a) = 0 for a = g d^2 / (2 v^2).
            var a = gravity * distance * distance / (2.0f * speed * speed);
            var c = dz + a;
            var discriminant = distance * distance - 4.0f * a * c;
            if (discriminant < 0.0f)
            {
                return false;
            }

            // The root of the minus sign is found as 2c / (d + root): the plain form subtracts two near numbers when a is small.
            var root = Trig.Sqrt(discriminant);
            var first = 2.0f * c / (distance + root);
            var second = (distance + root) / (2.0f * a);
            var tangent = first;
            if (high)
            {
                tangent = first > second ? first : second;
            }
            else
            {
                tangent = first < second ? first : second;
            }

            var cosine = 1.0f / Trig.Sqrt(1.0f + tangent * tangent);
            var along = speed * cosine / distance;
            velocity = new Vector3(dx * along, dy * along, speed * tangent * cosine);
            return true;
        }

        /// <summary>
        /// The speed that carries a thing from a point to another when launched at an angle above the ground (below it when
        /// negative), in degrees, under a gravity, or false when no speed does (the speed is then zero): a point at or
        /// above the line of the angle, an angle straight up or down or beyond, a point straight above or below, or no gravity.
        /// </summary>
        [LocalSafe]
        public static bool Speed(Vector3 from, Vector3 to, float angle, float gravity, out float speed)
        {
            speed = 0.0f;
            var dx = to.X - from.X;
            var dy = to.Y - from.Y;
            var dz = to.Z - from.Z;
            var distance = Trig.Sqrt(dx * dx + dy * dy);
            var cosine = Angles.Cos(angle);
            if (cosine <= Steep)
            {
                return false;
            }

            var sine = Angles.Sin(angle);

            // v^2 = g d^2 / (2 cos^2 (d tan - dz)), with cos^2 (d tan - dz) written as cos (d sin - dz cos).
            var below = 2.0f * cosine * (distance * sine - dz * cosine);
            var squared = gravity * distance * distance / below;
            if (below <= 0.0f || squared <= 0.0f)
            {
                return false;
            }

            speed = Trig.Sqrt(squared);
            return true;
        }

        /// <summary>The velocity of a vertical shot to a height above or below the start, at a speed: up while the height is within what the speed climbs to, down while it is within what a fall at that speed reaches.</summary>
        private static bool Straight(float dz, float speed, float gravity, out Vector3 velocity)
        {
            velocity = Vector3.Zero;
            if (dz == 0.0f)
            {
                return false;
            }

            var reach = speed * speed / (2.0f * gravity);
            if (dz > 0.0f)
            {
                if (gravity > 0.0f && dz > reach)
                {
                    return false;
                }

                velocity = new Vector3(0.0f, 0.0f, speed);
                return true;
            }

            if (gravity < 0.0f && dz < reach)
            {
                return false;
            }

            velocity = new Vector3(0.0f, 0.0f, -speed);
            return true;
        }
    }
}
