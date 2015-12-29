window.Util = (function() {
  var util = {};

  util.FormatTime = function(unixSec) {
    return (new Date(unixSec * 1000)).toLocaleString();
  }

  function M() {
    this.data_ = {};
    this.size_ = 0;
  }
  M.prototype.get = function(k) {
    return this.data_[k];
  }
  M.prototype.size = function() {
    return this.size_;
  }
  M.prototype.set = function(k, v) {
    if (!(k in this.data_)) {
      ++this.size_;
    }
    this.data_[k] = v;
  }
  M.prototype.keys = function() {
    var ks = [];
    for (var k in this.data_) {
      ks.push(k)
    }
    return ks;
  }
  util.Map = M;

  return util;
})();
