namespace Moonwell.Testing
{
    /// <summary>A buff that writes "buff pushed", "buff slammed" and "buff grounded" to the test log; it lasts until it is removed.</summary>
    public class PushGuard : Buff, IPushTarget
    {
        /// <summary>Makes a buff that lasts until it is removed.</summary>
        public PushGuard()
        {
            Duration = 0.0f;
        }

        /// <summary>Logs the push.</summary>
        public void OnPushed(Unit? pusher) => TestLog.Lines.Add("buff pushed");

        /// <summary>Logs the slam.</summary>
        public void OnSlammed(Slam slam) => TestLog.Lines.Add("buff slammed");

        /// <summary>Logs the landing.</summary>
        public void OnGrounded(float speed) => TestLog.Lines.Add("buff grounded");
    }
}
