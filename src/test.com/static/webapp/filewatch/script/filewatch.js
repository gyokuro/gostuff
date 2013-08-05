var ws; // WebSocket connection
var conn; // TCP connection
var host;
var port;
var service; // host:port



$(function () {

    /*** High hopes for drag and drop of directory to set which directory to watch...

	 $.event.props.push('dataTransfer');
	 $('#dropzone').on("dragenter dragstart dragend dragleave dragover drag drop",
	 function (e) {
	 e.preventDefault();
	 });
	 $('#dropzone').on({
	 dragenter: function (e) {
	 $(this).css('background-color', '#ffd1ff');
	 },
	 dragleave: function (e) {
	 $(this).css('background-color', '');
	 },
	 drop: function (e) {
	 e.stopPropagation();
	 e.preventDefault();

	 var file = e.dataTransfer.items[0].webkitGetAsEntry();

	 // NOTE: the drop event doesn't provide the full path of the directory!
	 console.log(['drop', e.dataTransfer.files, file ]);
	 if (file.isDirectory) {
	 // directory
	 }
	 }
	 });
	 **  Dashed hopes.  What a fk-ed up HTML5 spec */

    $('#disconnect-button').click(function() {
	if (ws != null) {
	    console.log(['disconnect', ws ])
	    ws.close()
	}
    })

    $("#connect-button").click(function () {
	host = $("#host").val();
	port = $("#port").val();
	event = $("#event").val();
	subscription = $("#subscription").val();

	if ((port == null) || (port == "")) {
	    port = "7777";
	}
	if (ws != null) {
	    ws.close();
	}
	if (host == null || host == "") {
	    host = "localhost";
	}
	if (subscription == null || subscription == "") {
	    subscription = ".*";
	}
	if (event == null || event == "") {
	    event = ".*";
	}
	service = host + ":" + port;
	ws = new WebSocket("ws://" + service + "/websocket/filewatch/?subscription=" + subscription +
			  "&event=" + event);

	if (ws == null) {
	    console.log("WebSocket creation failed");
	    return;
	} else {
	    console.log("WebSocket creation succeeded");
	}
	$("#console").text("");
	ws.onopen = function (event) {
	    console.log("onopen: service=" + service);
	    SetStatus("Watching...");
	}
	ws.onerror = function (event) {
	    console.log("onerror");
	    ws.close();
	    ws = null;
	}
	ws.onmessage = function (event) {
	    eventObj = JSON.parse(event.data)
	    console.log(["onmessage", event.data, eventObj]);

	    var color = 'blue';
	    if (eventObj.create) {
		color = 'green';
	    } else if (eventObj.deleted) {
		color = 'red';
	    } else if (eventObj.rename) {
		color = 'yellow';
	    }

	    $("#console").append("<div style='color:" + color + "';'>" +
				 htmlEncode(event.data) + "</div>");
	}
	ws.onclose = function (event) {
	    console.log("onclose");
	    SetStatus("Disconnected");
	    ws.close();
	    ws = null;
	}
    });

    function SetStatus(str) {
	$("#status").text(str);
    }

    function htmlEncode(value){
	return $('<div/>').text(value).html();
    }

    // get server info
    jQuery.get("/filewatch/_info", function(data, textStatus, jqXHR) {
	console.log([ 'config', data ])
	config = JSON.parse(data);
	$('#agent-info').html(data);
    });
});
