using System;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases from the full audit's raycast finding MT1. A map's bounce loop moves its object to the hit's point and
    /// bounces it there, so the next step's raycast starts on the surface: it must not hit again, or the object sticks. Each
    /// shape is met head on at a point a float holds exactly (where it used to stick for certain), then from 72 directions
    /// far from (0, 0), where the hit's point rounds to either side of the surface.
    /// </summary>
    public static class BounceCases
    {
        /// <summary>The circle's shape number for Cast.</summary>
        private const int CircleShape = 0;

        /// <summary>The bounds' shape number for Cast.</summary>
        private const int BoxShape = 1;

        /// <summary>The polygon's shape number for Cast.</summary>
        private const int PolygonShape = 2;

        /// <summary>The wall's shape number for Cast.</summary>
        private const int WallShape = 3;

        /// <summary>Where the sweeps' shapes are: far enough from (0, 0) that a float's step there is about 0.0005.</summary>
        private static Vector2 Far()
        {
            return new Vector2(5003.37f, -6011.59f);
        }

        /// <summary>The first hit of a step against one of the sweep's shapes, each about 50 across around Far().</summary>
        private static bool Cast(int shape, Vector2 from, Vector2 to, out RayHit hit)
        {
            var c = Far();
            if (shape == CircleShape)
            {
                return Raycast.Circle(from, to, c, 37.3f, out hit);
            }

            if (shape == BoxShape)
            {
                return Raycast.Bounds(from, to, new Bounds(c + new Vector2(-31.7f, -23.9f), c + new Vector2(27.3f, 29.1f)), out hit);
            }

            if (shape == PolygonShape)
            {
                return Raycast.Polygon(from, to, Pentagon(c), out hit);
            }

            return Raycast.Segment(from, to, c + new Vector2(-41.3f, -17.9f), c + new Vector2(37.1f, 23.7f), out hit);
        }

        /// <summary>An irregular pentagon around a point, about 70 across.</summary>
        internal static Polygon Pentagon(Vector2 c)
        {
            return new Polygon(
                c + new Vector2(31.3f, -7.1f), c + new Vector2(17.9f, 29.3f), c + new Vector2(-23.7f, 33.1f),
                c + new Vector2(-37.9f, -11.3f), c + new Vector2(-3.1f, -36.7f));
        }

        /// <summary>Runs a point at a shape for 60 steps of 7 from 200 away, aimed at its middle from a direction, a little to the side; gives the point.</summary>
        private static Bouncer Sweep(int shape, float degrees, float side)
        {
            var c = Far();
            var start = c + Vector2.FromAngle(degrees, 200f) + Vector2.FromAngle(degrees + 90f, side);
            var ball = new Bouncer(start, Vector2.FromAngle(degrees + 180f, 7f));
            for (var i = 0; i < 60; i++)
            {
                ball.Step(Cast(shape, ball.Position, ball.Position + ball.Velocity, out var hit), hit);
            }

            return ball;
        }

        /// <summary>From 72 directions, a point aimed at a shape hits it once, and goes on: at the end it is far from the shape and was never held still.</summary>
        private static void SweepShape(int shape, string what)
        {
            for (var k = 0; k < 72; k++)
            {
                var degrees = k * 5f + 0.37f;
                var ball = Sweep(shape, degrees, (k % 7 - 3) * 3.1f);
                var at = what + " from " + degrees;
                Expect.Equal(1, ball.Hits, at + ": hits");
                Expect.Equal(0, ball.Stuck, at + ": steps that did not move");
                Expect.True(ball.Position.DistanceTo(Far()) > 150f, at + ": went on, ended " + ball.Position.DistanceTo(Far()) + " from the middle");
            }
        }

        /// <summary>A circle met head on at a point a float holds exactly, (-4, 3) on a circle of 5: the bounce goes on.</summary>
        public static void CircleExact(FakeWorld world)
        {
            var ball = new Bouncer(new Vector2(-12f, 3f), new Vector2(10f, 0f));
            for (var i = 0; i < 10; i++)
            {
                ball.Step(Raycast.Circle(ball.Position, ball.Position + ball.Velocity, Vector2.Zero, 5f, out var hit), hit);
            }

            Expect.Equal(1, ball.Hits, "hits");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            // At (-4, 3) the normal is (-0.8, 0.6): (10, 0) bounces to (-2.8, 9.6), for 9 steps after the hit's.
            Vector2Cases.Near(new Vector2(-4f - 2.8f * 9f, 3f + 9.6f * 9f), ball.Position, "where it went");
        }

        /// <summary>Bounds met at (0, 53), on the left side: the bounce goes on.</summary>
        public static void BoundsExact(FakeWorld world)
        {
            var box = new Bounds(Vector2.Zero, new Vector2(100f, 100f));
            var ball = new Bouncer(new Vector2(-12f, 50f), new Vector2(8f, 2f));
            for (var i = 0; i < 10; i++)
            {
                ball.Step(Raycast.Bounds(ball.Position, ball.Position + ball.Velocity, box, out var hit), hit);
            }

            Expect.Equal(1, ball.Hits, "hits");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            Vector2Cases.Near(new Vector2(-8f * 8f, 53f + 2f * 8f), ball.Position, "where it went");
        }

        /// <summary>A square polygon met at (0, 53), on its left side: the bounce goes on.</summary>
        public static void PolygonExact(FakeWorld world)
        {
            var square = new Polygon(Vector2.Zero, new Vector2(100f, 0f), new Vector2(100f, 100f), new Vector2(0f, 100f));
            var ball = new Bouncer(new Vector2(-12f, 50f), new Vector2(8f, 2f));
            for (var i = 0; i < 10; i++)
            {
                ball.Step(Raycast.Polygon(ball.Position, ball.Position + ball.Velocity, square, out var hit), hit);
            }

            Expect.Equal(1, ball.Hits, "hits");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            Vector2Cases.Near(new Vector2(-8f * 8f, 53f + 2f * 8f), ball.Position, "where it went");
        }

        /// <summary>A wall met at (0, 3): the bounce goes on (it used to hit again at -0).</summary>
        public static void WallExact(FakeWorld world)
        {
            var ball = new Bouncer(new Vector2(-12f, 1f), new Vector2(8f, 1f));
            for (var i = 0; i < 10; i++)
            {
                ball.Step(Raycast.Segment(ball.Position, ball.Position + ball.Velocity, new Vector2(0f, -50f), new Vector2(0f, 50f), out var hit), hit);
            }

            Expect.Equal(1, ball.Hits, "hits");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            Vector2Cases.Near(new Vector2(-8f * 8f, 2.5f + 8f), ball.Position, "where it went");
        }

        /// <summary>A circle far from (0, 0), from 72 directions.</summary>
        public static void CircleSweep(FakeWorld world)
        {
            SweepShape(CircleShape, "circle");
        }

        /// <summary>Bounds far from (0, 0), from 72 directions.</summary>
        public static void BoundsSweep(FakeWorld world)
        {
            SweepShape(BoxShape, "bounds");
        }

        /// <summary>A pentagon far from (0, 0), from 72 directions.</summary>
        public static void PolygonSweep(FakeWorld world)
        {
            SweepShape(PolygonShape, "polygon");
        }

        /// <summary>A wall far from (0, 0), from the 72 directions that meet it (a wall met edge on is passed by).</summary>
        public static void WallSweep(FakeWorld world)
        {
            for (var k = 0; k < 72; k++)
            {
                var degrees = k * 5f + 0.37f;
                var ball = Sweep(WallShape, degrees, (k % 7 - 3) * 3.1f);
                var at = "wall from " + degrees;
                Expect.True(ball.Hits <= 1, at + ": hits " + ball.Hits);
                Expect.Equal(0, ball.Stuck, at + ": steps that did not move");
                Expect.True(ball.Travelled > 400f, at + ": went on, travelled " + ball.Travelled);
            }
        }

        /// <summary>
        /// A point inside a room of four walls far from (0, 0) bounces off them for 2,000 steps: it never leaves the room and
        /// never stops.
        /// </summary>
        public static void Room(FakeWorld world)
        {
            var c = Far();
            var a = c + new Vector2(-150.3f, -110.7f);
            var b = c + new Vector2(170.9f, -95.1f);
            var d = c + new Vector2(140.1f, 130.3f);
            var e = c + new Vector2(-160.7f, 120.9f);
            var room = new Polygon(a, b, d, e);
            var ball = new Bouncer(c, new Vector2(6.13f, 2.71f));
            for (var i = 0; i < 2000; i++)
            {
                var from = ball.Position;
                var to = from + ball.Velocity;
                var best = 2f;
                var nearest = default(RayHit);
                Nearest(Raycast.Segment(from, to, a, b, out var hit), hit, ref best, ref nearest);
                Nearest(Raycast.Segment(from, to, b, d, out hit), hit, ref best, ref nearest);
                Nearest(Raycast.Segment(from, to, d, e, out hit), hit, ref best, ref nearest);
                Nearest(Raycast.Segment(from, to, e, a, out hit), hit, ref best, ref nearest);
                ball.Step(best <= 1f, nearest);
                Expect.True(room.Contains(ball.Position), "inside the room after step " + i + ", at " + ball.Position);
            }

            Expect.True(ball.Hits > 30, "bounced " + ball.Hits + " times");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
        }

        /// <summary>Keeps a hit if it is the nearest so far.</summary>
        private static void Nearest(bool did, RayHit hit, ref float best, ref RayHit nearest)
        {
            if (did && hit.Fraction < best)
            {
                best = hit.Fraction;
                nearest = hit;
            }
        }

        /// <summary>
        /// A point that meets the L shape's notch (scaled by 100, far from (0, 0)) bounces off its side and then its floor,
        /// and goes on: it never stops, is never inside, and leaves the notch.
        /// </summary>
        public static void Notch(FakeWorld world)
        {
            var c = Far();
            var l = new Polygon(
                c, c + new Vector2(1000f, 0f), c + new Vector2(1000f, 400f),
                c + new Vector2(400f, 400f), c + new Vector2(400f, 1000f), c + new Vector2(0f, 1000f));
            var ball = new Bouncer(c + new Vector2(970.3f, 700.7f), new Vector2(-23.9f, -11.3f));
            for (var i = 0; i < 400; i++)
            {
                ball.Step(Raycast.Polygon(ball.Position, ball.Position + ball.Velocity, l, out var hit), hit);
                var p = ball.Position - c;
                // Inside the notch's floor or sides by more than the 0.01 a surface counts as is inside the shape.
                Expect.True(p.Y > 399.98f && (p.X > 399.98f || p.Y > 999.98f), "out of the shape after step " + i + ", at " + p);
            }

            Expect.Equal(2, ball.Hits, "bounces, off the side and the floor");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            Expect.True(ball.Position.DistanceTo(c) > 2000f, "left the notch, at " + (ball.Position - c));
        }

        /// <summary>A point that meets the top of bounds and slides along it goes on along the top and off its end.</summary>
        public static void SlideBounds(FakeWorld world)
        {
            var c = Far();
            var box = new Bounds(c, c + new Vector2(300f, 100f));
            var ball = new Bouncer(c + new Vector2(10.3f, 130.7f), new Vector2(9.1f, -7.3f));
            ball.Slides = true;
            for (var i = 0; i < 60; i++)
            {
                ball.Step(Raycast.Bounds(ball.Position, ball.Position + ball.Velocity, box, out var hit), hit);
            }

            Expect.Equal(1, ball.Hits, "hits");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            Expect.True(ball.Position.X - c.X > 300f, "slid along the top and past its end, at " + (ball.Position - c));
        }

        /// <summary>A point that meets a slanted side of a polygon and slides along it goes on along it and off its end.</summary>
        public static void SlidePolygon(FakeWorld world)
        {
            var c = Far();
            var ramp = new Polygon(c, c + new Vector2(400f, 0f), c + new Vector2(400f, 130.7f));
            var ball = new Bouncer(c + new Vector2(30.3f, 90.1f), new Vector2(6.1f, -9.7f));
            ball.Slides = true;
            for (var i = 0; i < 200; i++)
            {
                ball.Step(Raycast.Polygon(ball.Position, ball.Position + ball.Velocity, ramp, out var hit), hit);
                Expect.True(!ramp.Contains(ball.Position) || ramp.Bounds.Contains(ball.Position) && Near(ramp, ball.Position), "out of the ramp after step " + i);
            }

            Expect.Equal(1, ball.Hits, "hits");
            Expect.Equal(0, ball.Stuck, "steps that did not move");
            Expect.True(ball.Position.X - c.X > 400f, "slid up the ramp and past its end, at " + (ball.Position - c));
        }

        /// <summary>Whether a point is within 0.02 of the ramp's slanted side.</summary>
        private static bool Near(Polygon ramp, Vector2 point)
        {
            var a = ramp.Points[0];
            var b = ramp.Points[2];
            var side = b - a;
            return MathF.Abs((point - a).Cross(side)) / side.Length < 0.02f;
        }

        /// <summary>A point on a surface that moves into the shape still hits it at its start, with the surface's normal; one that starts deeper in hits at its start with the normal against its direction, as before.</summary>
        public static void IntoFromSurface(FakeWorld world)
        {
            var box = new Bounds(Vector2.Zero, new Vector2(100f, 100f));
            Expect.True(Raycast.Bounds(new Vector2(0f, 50f), new Vector2(8f, 52f), box, out var hit), "bounds: hits");
            Expect.Near(0f, hit.Fraction, "bounds: fraction");
            Vector2Cases.Near(new Vector2(0f, 50f), hit.Point, "bounds: point");
            Vector2Cases.Near(new Vector2(-1f, 0f), hit.Normal, "bounds: the side's normal");
            var square = new Polygon(Vector2.Zero, new Vector2(100f, 0f), new Vector2(100f, 100f), new Vector2(0f, 100f));
            Expect.True(Raycast.Polygon(new Vector2(0f, 50f), new Vector2(8f, 52f), square, out hit), "polygon: hits");
            Expect.Near(0f, hit.Fraction, "polygon: fraction");
            Vector2Cases.Near(new Vector2(-1f, 0f), hit.Normal, "polygon: the side's normal");
            Expect.True(Raycast.Circle(new Vector2(-4f, 3f), new Vector2(4f, 3f), Vector2.Zero, 5f, out hit), "circle: hits");
            Expect.Near(0f, hit.Fraction, "circle: fraction");
            Vector2Cases.Near(new Vector2(-0.8f, 0.6f), hit.Normal, "circle: the surface's normal");
            Expect.True(Raycast.Bounds(new Vector2(50f, 50f), new Vector2(58f, 52f), box, out hit), "deep in bounds: hits");
            Expect.Near(0f, hit.Fraction, "deep in bounds: fraction");
            Vector2Cases.Near(new Vector2(-8f, -2f).Normalized(), hit.Normal, "deep in bounds: against the direction");
            Expect.True(!Raycast.Bounds(new Vector2(0f, 50f), new Vector2(0f, 60f), box, out _), "along the side: no hit");
            Expect.True(!Raycast.Polygon(new Vector2(0f, 50f), new Vector2(0f, 60f), square, out _), "along the polygon's side: no hit");
            Expect.True(!Raycast.Bounds(new Vector2(0f, 50f), new Vector2(-8f, 52f), box, out _), "out of the side: no hit");
            Expect.True(!Raycast.Segment(new Vector2(0f, 3f), new Vector2(8f, 5f), new Vector2(0f, -50f), new Vector2(0f, 50f), out _), "from on a wall, either way: no hit");
            Expect.True(!Raycast.Segment(new Vector2(0f, 3f), new Vector2(-8f, 5f), new Vector2(0f, -50f), new Vector2(0f, 50f), out _), "from on a wall, the other way: no hit");
        }

        /// <summary>
        /// A point pulled toward a slope at every step, as by gravity, slides down along it when a step goes on after a hit
        /// with the new velocity for the rest of the step, as the guide's loop does. Moved only to the hit's point, it hits at
        /// the start of every step and stands where it landed: the 0.3 gate saw that, in the game and in desktop Lua.
        /// </summary>
        public static void SlideUnderGravity(FakeWorld world)
        {
            const float step = 1f / 32f;
            var ramp = new Polygon(new Vector2(-500f, 250f), new Vector2(300f, -150f), new Vector2(-500f, -150f));
            var outward = new Vector2(0.4472136f, 0.8944272f);
            var position = new Vector2(-400f, 550f);
            var velocity = Vector2.Zero;
            var landed = false;
            var landedX = 0f;
            for (var i = 0; i < 96; i++)
            {
                velocity = velocity + new Vector2(0f, -900f * step);
                var left = step;
                for (var hits = 0; hits < 4 && left > 0f; hits++)
                {
                    var to = position + velocity * left;
                    if (!Raycast.Polygon(position, to, ramp, out var hit))
                    {
                        position = to;
                        break;
                    }

                    position = hit.Point;
                    velocity = velocity.Slide(hit.Normal);
                    left *= 1f - hit.Fraction;
                    if (!landed)
                    {
                        landed = true;
                        landedX = position.X;
                    }
                }

                // On the slope is not in it: 0.1 off the slope, outward, is outside.
                Expect.True(!ramp.Contains(position + outward * 0.1f), "never in the ramp, at step " + i + ": " + position);
            }

            Expect.True(landed, "it landed on the slope");
            Expect.True(position.X - landedX > 300f, "slid on down the slope: " + (position.X - landedX) + " along x from where it landed");
        }
    }
}
