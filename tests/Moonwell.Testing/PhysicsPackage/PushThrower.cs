using System;

namespace Moonwell.Testing
{
    /// <summary>A unit component whose OnPushed throws, so the push cases can see a failing reaction reported and the others run.</summary>
    public class PushThrower : UnitComponent, IPushTarget
    {
        /// <summary>Logs, then throws.</summary>
        public void OnPushed(Unit? pusher)
        {
            TestLog.Lines.Add("thrower pushed");
            throw new InvalidOperationException("thrown on purpose");
        }
    }
}
