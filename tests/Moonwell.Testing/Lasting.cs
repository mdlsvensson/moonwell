namespace Moonwell.Testing
{
    /// <summary>A buff with no duration: it stays on its unit until it is removed.</summary>
    public class Lasting : Buff
    {
        /// <summary>Makes a buff that lasts until it is removed.</summary>
        public Lasting()
        {
            Duration = 0f;
        }
    }
}
