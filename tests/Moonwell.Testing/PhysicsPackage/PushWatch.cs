namespace Moonwell.Testing
{
    /// <summary>A unit component that writes "component pushed" and "component slammed" to the test log; it writes no OnGrounded, so the package must not call one.</summary>
    public class PushWatch : UnitComponent, IPushTarget
    {
        /// <summary>Logs the push.</summary>
        public void OnPushed(Unit? pusher) => TestLog.Lines.Add("component pushed");

        /// <summary>Logs the slam.</summary>
        public void OnSlammed(Slam slam) => TestLog.Lines.Add("component slammed");
    }
}
