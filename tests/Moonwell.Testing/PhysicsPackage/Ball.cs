using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A missile the ground cases make: a case sets its settings, and it records its impacts and its rests, and runs an action a case gives it at an impact.</summary>
    public sealed class Ball : Missile
    {
        /// <summary>How many times OnImpact ran.</summary>
        public int Impacts;

        /// <summary>The impact OnImpact last had; null before one.</summary>
        public Impact? Last;

        /// <summary>The kind, place, normal, speed and contact of the impact OnImpact last had, read inside OnImpact: kind, x, y, z, nx, ny, nz, speed.</summary>
        public float[] Read = new float[8];

        /// <summary>The slowest closing speed of its impacts; a billion before one.</summary>
        public float Slowest = 1000000000.0f;

        /// <summary>The owner that was current when OnImpact last ran.</summary>
        public Scope? ImpactedWith;

        /// <summary>Whether it had ended when OnImpact last ran.</summary>
        public bool EndedInImpact;

        /// <summary>How many times OnRest ran.</summary>
        public int Rested;

        /// <summary>The owner that was current when OnRest last ran.</summary>
        public Scope? RestedWith;

        /// <summary>Where the reactions write their names, in the order they ran; null for nowhere.</summary>
        public List<string>? Log;

        /// <summary>An action OnImpact runs after it has recorded; null for none.</summary>
        public Action<Impact>? Impacting;

        /// <summary>Records the impact, the current owner and whether it has ended, then runs the action.</summary>
        public override void OnImpact(Impact impact)
        {
            Impacts++;
            Last = impact;
            Read[0] = (float)(int)impact.Kind;
            Read[1] = impact.X;
            Read[2] = impact.Y;
            Read[3] = impact.Z;
            var normal = impact.Normal;
            Read[4] = normal.X;
            Read[5] = normal.Y;
            Read[6] = normal.Z;
            Read[7] = impact.Speed;
            if (impact.Speed < Slowest)
            {
                Slowest = impact.Speed;
            }
            ImpactedWith = Scope.Current;
            EndedInImpact = HasEnded;
            if (Log != null)
            {
                Log.Add("ball.impact");
            }

            if (Impacting != null)
            {
                Impacting(impact);
            }
        }

        /// <summary>Counts the rest and keeps the current owner.</summary>
        public override void OnRest()
        {
            Rested++;
            RestedWith = Scope.Current;
            if (Log != null)
            {
                Log.Add("ball.rest");
            }
        }
    }
}
