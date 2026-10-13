// The landing page only needs these exports from anime.js 4.1.4.
// See docs/development/site-performance.md for the reproducible bundle command.
import { animate, createTimeline, stagger, utils } from "animejs";

window.anime = { animate, createTimeline, stagger, utils: { round: utils.round } };
