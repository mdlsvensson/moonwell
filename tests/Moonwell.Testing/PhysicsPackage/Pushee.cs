using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A unit the push cases make: it writes "unit pushed", "unit slammed" and "unit landed" to the test log, and records the slams, the landings, the pushers and the owner current at each reaction. It starts with a PushWatch.</summary>
    public class Pushee : Unit, IPushTarget
    {
        /// <summary>The slams, in the order they came.</summary>
        public List<Slam> Slams = new List<Slam>();

        /// <summary>The speed of each slam, read inside OnSlammed.</summary>
        public List<float> SlamSpeeds = new List<float>();

        /// <summary>The pusher of each slam, read inside OnSlammed; null for none.</summary>
        public List<Unit?> SlamPushers = new List<Unit?>();

        /// <summary>The speed of each landing.</summary>
        public List<float> Landings = new List<float>();

        /// <summary>The pusher of each push.</summary>
        public List<Unit?> Pushers = new List<Unit?>();

        /// <summary>The owner current at the last reaction.</summary>
        public Owner? RanWith;

        /// <summary>Makes a unit with its component.</summary>
        public Pushee()
        {
            Components.Add(new PushWatch());
        }

        /// <summary>Logs the push and its pusher.</summary>
        public void OnPushed(Unit? pusher)
        {
            TestLog.Lines.Add("unit pushed");
            Pushers.Add(pusher);
            RanWith = Owner.Current;
        }

        /// <summary>Logs the slam and what it read.</summary>
        public void OnSlammed(Slam slam)
        {
            TestLog.Lines.Add("unit slammed");
            Slams.Add(slam);
            SlamSpeeds.Add(slam.Speed);
            SlamPushers.Add(slam.Pusher);
            RanWith = Owner.Current;
        }

        /// <summary>Logs the landing and its speed.</summary>
        public void OnLanded(float speed)
        {
            TestLog.Lines.Add("unit landed");
            Landings.Add(speed);
            RanWith = Owner.Current;
        }
    }
}
