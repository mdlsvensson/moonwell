namespace Moonwell.Testing
{
    /// <summary>A buff that lasts 3 seconds and ticks every second, and logs each hook it runs.</summary>
    public class Burn : Buff
    {
        /// <summary>Makes a burn: 3 seconds, a tick each second.</summary>
        public Burn()
        {
            Duration = 3f;
            Tick = 1f;
        }

        /// <summary>Logs that the burn was added to its unit.</summary>
        protected override void OnAdded()
        {
            TestLog.Lines.Add("added");
        }

        /// <summary>Logs that the burn was applied.</summary>
        public override void OnApply()
        {
            TestLog.Lines.Add("apply");
        }

        /// <summary>Logs that the burn ticked.</summary>
        public override void OnTick()
        {
            TestLog.Lines.Add("tick");
        }

        /// <summary>Logs that the burn expired.</summary>
        public override void OnExpire()
        {
            TestLog.Lines.Add("expire");
        }

        /// <summary>Logs that the burn was removed.</summary>
        protected override void OnRemoved()
        {
            TestLog.Lines.Add("removed");
        }
    }
}
