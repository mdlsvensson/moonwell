using System;

namespace Moonwell.Testing
{
    /// <summary>Cases for the physics package's settings and for Ballistics, whose answers are checked by flying them.</summary>
    public static class BallisticsCases
    {
        /// <summary>How close to the point a launch must pass, in game units.</summary>
        private const float Reach = 1.0f;

        /// <summary>
        /// The closest a thing launched from a point with a velocity comes to a target, flown with exact steps of 1/2048 s:
        /// each position is the closed form, start plus velocity times t, less half gravity times t squared in height. The
        /// flight is followed for 4 seconds.
        /// </summary>
        private static float ClosestPass(Vector3 from, Vector3 velocity, float gravity, Vector3 to)
        {
            var best = float.MaxValue;
            for (var step = 0; step <= 8192; step++)
            {
                var t = step / 2048.0f;
                var dx = from.X + velocity.X * t - to.X;
                var dy = from.Y + velocity.Y * t - to.Y;
                var dz = from.Z + velocity.Z * t - 0.5f * gravity * t * t - to.Z;
                var distance = MathF.Sqrt(dx * dx + dy * dy + dz * dz);
                if (distance < best)
                {
                    best = distance;
                }
            }

            return best;
        }

        /// <summary>Fails unless the launch, low or high, passes within Reach of the point and has the speed.</summary>
        private static void Hits(Vector3 from, Vector3 to, float speed, bool high, float gravity, string what)
        {
            Vector3 velocity;
            Expect.True(Ballistics.Velocity(from, to, speed, high, gravity, out velocity), what + ": a launch is found");
            Expect.True(Math.Abs(velocity.Length - speed) < 0.01f, what + ": it has the speed");
            var pass = ClosestPass(from, velocity, gravity, to);
            Expect.True(pass < Reach, what + ": it passes within " + Reach + " of the point, passes by " + pass);
        }

        /// <summary>Gravity is 800 until a map sets it; the setting is forgotten between cases.</summary>
        public static void GravitySetting(FakeWorld world)
        {
            Expect.Near(800.0f, Physics.Gravity, "the default gravity");
            Physics.Gravity = 400.0f;
            Expect.Near(400.0f, Physics.Gravity, "a set gravity");
            Physics.ResetSettings();
            Expect.Near(800.0f, Physics.Gravity, "reset gives the default");
        }

        /// <summary>The package is installed by Install on a fake world, hands that world to its users, and is installed afresh when the interactions package is installed again.</summary>
        public static void Install(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Expect.True(!Physics.IsInstalled, "not installed before Install");
            Physics.Install(fake);
            Expect.True(Physics.IsInstalled, "installed after Install");
            Expect.Same(fake, Physics.World, "the world is the fake");
            Physics.EnsureInstalled();
            Expect.Same(fake, Physics.World, "EnsureInstalled leaves an installed package alone");
            Interactions.Install(new FakeInteractionWorld());
            Expect.True(!Physics.IsInstalled, "installing interactions again clears the physics hook");
            Physics.Install(fake);
            Expect.True(Physics.IsInstalled, "installed again");
        }

        /// <summary>From (0, 0, 0) to (800, 0, 0) at 1,000 with gravity 800, the low and the high launch both pass within 1 of the point.</summary>
        public static void VelocityHits(FakeWorld world)
        {
            var from = new Vector3(0.0f, 0.0f, 0.0f);
            var to = new Vector3(800.0f, 0.0f, 0.0f);
            Hits(from, to, 1000.0f, false, 800.0f, "low");
            Hits(from, to, 1000.0f, true, 800.0f, "high");
            Vector3 low;
            Vector3 high;
            Ballistics.Velocity(from, to, 1000.0f, false, 800.0f, out low);
            Ballistics.Velocity(from, to, 1000.0f, true, 800.0f, out high);
            Expect.True(high.Z > low.Z, "the high launch rises more steeply");
            Expect.True(low.Y == 0.0f && high.Y == 0.0f, "both stay in the plane of the shot");
        }

        /// <summary>A target off the axis, above and below the launch, is hit by both launches.</summary>
        public static void VelocitySlanted(FakeWorld world)
        {
            var from = new Vector3(100.0f, 200.0f, 50.0f);
            var up = new Vector3(400.0f, -200.0f, 300.0f);
            var down = new Vector3(-500.0f, 600.0f, -250.0f);
            Hits(from, up, 900.0f, false, 800.0f, "up low");
            Hits(from, up, 900.0f, true, 800.0f, "up high");
            Hits(from, down, 900.0f, false, 800.0f, "down low");
            Hits(from, down, 900.0f, true, 800.0f, "down high");
            Hits(from, up, 900.0f, false, 300.0f, "up low, a weaker gravity");
        }

        /// <summary>A target farther than the speed can carry is out of reach: no launch, and the velocity is left zero.</summary>
        public static void OutOfReach(FakeWorld world)
        {
            Vector3 velocity;
            var found = Ballistics.Velocity(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(5000.0f, 0.0f, 0.0f), 100.0f, false, 800.0f, out velocity);
            Expect.True(!found, "100 a second does not reach 5,000");
            Expect.True(velocity.X == 0.0f && velocity.Y == 0.0f && velocity.Z == 0.0f, "no velocity is given");
            found = Ballistics.Velocity(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(5000.0f, 0.0f, 0.0f), 100.0f, true, 800.0f, out velocity);
            Expect.True(!found, "nor does the high launch");
            found = Ballistics.Velocity(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(100.0f, 0.0f, 0.0f), 0.0f, false, 800.0f, out velocity);
            Expect.True(!found, "no speed reaches nothing");
            found = Ballistics.Velocity(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(100.0f, 0.0f, 0.0f), -50.0f, false, 800.0f, out velocity);
            Expect.True(!found, "a negative speed reaches nothing");
        }

        /// <summary>The speed for a 45 degree launch to (800, 0, 0) is 800.</summary>
        public static void SpeedForAngle(FakeWorld world)
        {
            float speed;
            var found = Ballistics.Speed(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(800.0f, 0.0f, 0.0f), 45.0f, 800.0f, out speed);
            Expect.True(found, "45 degrees reaches the point");
            Expect.True(Math.Abs(speed - 800.0f) < 0.5f, "the speed is 800, was " + speed);
        }

        /// <summary>A point above the line of the launch angle is never reached at that angle, and neither is anything along the ground by a launch straight up.</summary>
        public static void Above(FakeWorld world)
        {
            float speed;
            var found = Ballistics.Speed(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(800.0f, 0.0f, 1000.0f), 45.0f, 800.0f, out speed);
            Expect.True(!found, "a point above the 45 degree line is out of reach");
            Expect.True(speed == 0.0f, "no speed is given");
            found = Ballistics.Speed(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(800.0f, 0.0f, 800.0f), 45.0f, 800.0f, out speed);
            found = Ballistics.Speed(new Vector3(0.0f, 0.0f, 0.0f), new Vector3(800.0f, 0.0f, 0.0f), 90.0f, 800.0f, out speed);
            Expect.True(!found, "straight up goes nowhere along the ground");
        }

        /// <summary>The speed for an angle, put back into Velocity, gives a launch at that angle on one of the two arcs, and it hits.</summary>
        public static void SpeedThenVelocity(FakeWorld world)
        {
            var from = new Vector3(0.0f, 0.0f, 100.0f);
            var to = new Vector3(600.0f, 300.0f, 0.0f);
            float speed;
            Expect.True(Ballistics.Speed(from, to, 30.0f, 800.0f, out speed), "30 degrees reaches the point");
            Vector3 velocity;
            Expect.True(Ballistics.Velocity(from, to, speed, false, 800.0f, out velocity), "that speed reaches it");
            var horizontal = MathF.Sqrt(velocity.X * velocity.X + velocity.Y * velocity.Y);
            var slope = velocity.Z / horizontal;
            Expect.True(Math.Abs(slope - Angles.Sin(30.0f) / Angles.Cos(30.0f)) < 0.002f, "the low launch is at 30 degrees, slope " + slope);
            Hits(from, to, speed, false, 800.0f, "the speed for 30 degrees");
        }

        /// <summary>With no gravity the launch is straight at the point, the same low or high.</summary>
        public static void GravityZero(FakeWorld world)
        {
            var from = new Vector3(0.0f, 0.0f, 0.0f);
            var to = new Vector3(300.0f, 400.0f, 120.0f);
            var length = MathF.Sqrt(300.0f * 300.0f + 400.0f * 400.0f + 120.0f * 120.0f);
            Vector3 low;
            Vector3 high;
            Expect.True(Ballistics.Velocity(from, to, 500.0f, false, 0.0f, out low), "low is found");
            Expect.True(Ballistics.Velocity(from, to, 500.0f, true, 0.0f, out high), "high is found");
            Expect.True(Math.Abs(low.X - 300.0f / length * 500.0f) < 0.01f, "low x runs at the point");
            Expect.True(Math.Abs(low.Y - 400.0f / length * 500.0f) < 0.01f, "low y runs at the point");
            Expect.True(Math.Abs(low.Z - 120.0f / length * 500.0f) < 0.01f, "low z runs at the point");
            Expect.True(low.X == high.X && low.Y == high.Y && low.Z == high.Z, "high is ignored");
            Expect.True(Math.Abs(ClosestPass(from, low, 0.0f, to)) < Reach, "it passes through the point");
            Expect.True(!Ballistics.Velocity(from, from, 500.0f, false, 0.0f, out low), "a point on the start has no direction");
        }

        /// <summary>A target straight above or below is reached by a vertical launch while the height allows it.</summary>
        public static void Vertical(FakeWorld world)
        {
            var from = new Vector3(10.0f, 20.0f, 0.0f);
            Vector3 velocity;
            Expect.True(Ballistics.Velocity(from, new Vector3(10.0f, 20.0f, 100.0f), 500.0f, false, 800.0f, out velocity), "100 up is within reach of 500 a second");
            Expect.True(velocity.X == 0.0f && velocity.Y == 0.0f && Math.Abs(velocity.Z - 500.0f) < 0.001f, "it goes straight up at the speed");
            Expect.True(!Ballistics.Velocity(from, new Vector3(10.0f, 20.0f, 200.0f), 500.0f, false, 800.0f, out velocity), "200 up is above the peak of 156");
            Expect.True(Ballistics.Velocity(from, new Vector3(10.0f, 20.0f, -100.0f), 500.0f, false, 800.0f, out velocity), "100 down is within reach");
            Expect.True(velocity.X == 0.0f && velocity.Y == 0.0f && Math.Abs(velocity.Z + 500.0f) < 0.001f, "it goes straight down at the speed");
        }
    }
}
